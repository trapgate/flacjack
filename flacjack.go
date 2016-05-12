/*
	flacjack converts FLAC files to other formats, where "other formats"
	currently means MP3.

	Copyright (c) 2016 Geoff Hickey

	Permission is hereby granted, free of charge, to any person obtaining a copy
	of this software and associated documentation files (the "Software"), to
	deal in the Software without restriction, including without limitation the
	rights to use, copy, modify, merge, publish, distribute, sublicense, and/or
	sell copies of the Software, and to permit persons to whom the Software is
	furnished to do so, subject to the following conditions:

	The above copyright notice and this permission notice shall be included in
	all copies or substantial portions of the Software.

	THE SOFTWARE IS PROVIDED "AS IS", WITHOUT WARRANTY OF ANY KIND, EXPRESS OR
	IMPLIED, INCLUDING BUT NOT LIMITED TO THE WARRANTIES OF MERCHANTABILITY,
	FITNESS FOR A PARTICULAR PURPOSE AND NONINFRINGEMENT. IN NO EVENT SHALL THE
	AUTHORS OR COPYRIGHT HOLDERS BE LIABLE FOR ANY CLAIM, DAMAGES OR OTHER
	LIABILITY, WHETHER IN AN ACTION OF CONTRACT, TORT OR OTHERWISE, ARISING
	FROM, OUT OF OR IN CONNECTION WITH THE SOFTWARE OR THE USE OR OTHER DEALINGS
	IN THE SOFTWARE.
*/

package main

import (
	"bufio"
	"fmt"
	"io/ioutil"
	"log"
	"os"
	"os/exec"
	"path"
	"path/filepath"
	"runtime"
	"strings"

	"github.com/pborman/getopt"
)

// Progress represents how far along a file conversion is.
type Progress struct {
	gortn int    // The index of the goroutine
	file  string // Source file name (flac file)
	dest  string // Destination file name (mp3 file)
	stage string // Name of the current stage
	err   error  // Error information, if any error is encountered
}

func (p Progress) String() string {
	if len(p.stage) == 0 {
		return "Idle"
	}
	// The termcodes embedded here set the color of the stage to green.
	return fmt.Sprintf("%-60v \033[32m%15v\033[0m", path.Base(p.file), p.stage)
}

// ErrorMsg returns an error message if this translation failed
func (p Progress) ErrorMsg() string {
	if p.err == nil {
		return ""
	}
	return fmt.Sprintf("%v ERROR: %v", p.file, p.err)
}

// Command line flags
var inputpath, outputpath *string
var transch = make(chan string, 100)

func main() {
	inputpath = getopt.StringLong("input-path", 'i', "/mnt/music/flac",
		"the location of the FLAC files to convert")
	outputpath = getopt.StringLong("output-path", 'o', "/mnt/music/mp3",
		"where to put the translated files")
	help := getopt.BoolLong("help", 'h', "display this usage information")
	getopt.SetParameters("")
	getopt.Parse()
	if *help {
		getopt.Usage()
		os.Exit(0)
	}

	// flag.Parse()

	converters := runtime.GOMAXPROCS(0)

	fmt.Printf("Starting FLAC converter with %v routines\n", converters)

	// Channel for the converters to report progress to the main routine.
	outputch := make(chan Progress)
	// Channel for the converters to signal they're done.
	donech := make(chan int)
	// Array to hold the current progress for every routine.
	rtnProgress := make([]Progress, converters)

	// Make room for the goroutine status lines
	fmt.Print(strings.Repeat("\n", converters))

	// Start a goroutine to find the files we need to convert, and several more
	// to process the files.
	go findTranslatables(transch)
	for i := 0; i < converters; i++ {
		go convert(i, transch, outputch, donech)
	}

	for {
		select {
		case prog := <-outputch:
			rtnProgress[prog.gortn] = prog
			showProgress(rtnProgress)
		case <-donech:
			converters--
			if converters == 0 {
				return
			}
		}
	}
}

func showProgress(p []Progress) {
	fmt.Printf("\033[%vA\033[0G", len(p))
	// Print all errors first, so they don't get erased
	// TODO: emit termcodes only when stdout is a tty
	// TODO: clean up termcodes
	for _, prog := range p {
		if prog.err != nil {
			fmt.Printf("")
			// These termcodes clear the line and turn the errors red.
			fmt.Printf("\033[0K\033[31m%v\033[0K\033[0m\n", prog.ErrorMsg())
			fmt.Printf("")
		}
	}
	for ix, prog := range p {
		// Termcode to clear the line.
		fmt.Printf("\033[0K")
		fmt.Printf("%2v: %v\n", ix, prog)
	}
}

func findTranslatables(transch chan string) {
	// Close the channel on exit, which will shut down the program.
	defer close(transch)
	filepath.Walk(*inputpath, walker)
}

// This is called for each file in our directory walk.
func walker(path string, info os.FileInfo, err error) error {
	if err != nil {
		log.Printf("failed to read from directory %s\n", *inputpath)
		return err
	}
	if info.IsDir() {
		return nil
	}
	ext := filepath.Ext(path)
	if strings.ToLower(ext) != ".flac" {
		return nil
	}
	// Found a flac file. Now, is it already converted?
	ofile := mp3name(path)
	_, err = os.Stat(ofile)
	if os.IsNotExist(err) {
		transch <- path
	}

	return nil
}

// Return the full path of the output file. To build this path, strip the
// inputpath from the beginning of the input path, and replace it with the
// outputpath, and replace the extension with 'mp3'.
func mp3name(fn string) string {
	ext := filepath.Ext(fn)
	ofile := fn[len(*inputpath):len(fn)-len(ext)] + ".mp3"
	ofile = path.Join(*outputpath, ofile)
	return ofile
}

// A number of convert goroutines will run until the program finishes.
func convert(ix int, transch chan string, outputch chan Progress, donech chan int) {
	prog := new(Progress)
	prog.gortn = ix
	outputch <- *prog
	for {
		select {
		case file, more := <-transch:
			if more {
				convertFile(ix, file, outputch)
				outputch <- *prog
			} else {
				donech <- 1
				return
			}
		}
	}
}

// Convert a source file to the target format.
func convertFile(ix int, fn string, outputch chan Progress) {
	prog := new(Progress)
	// Get the tags
	prog.gortn = ix
	prog.stage = "Extracting tags"
	prog.file = fn
	prog.dest = mp3name(fn)
	outputch <- *prog
	tags, err := flacTags(fn)
	if err != nil {
		prog.err = err
		outputch <- *prog
		return
	}

	// Decode the FLAC file
	prog.stage = "Decode FLAC"
	outputch <- *prog
	wavfile, err := decodeFlac(fn)
	if err != nil {
		prog.err = err
		outputch <- *prog
		return
	}
	defer os.Remove(wavfile)

	// Encode the MP3
	prog.stage = "Encode MP3"
	outputch <- *prog
	err = encodeMp3(wavfile, mp3name(fn), tags)
	if err != nil {
		prog.err = err
		outputch <- *prog
		return
	}
}

// Get the tags from a flac file, in a dictionary.
func flacTags(flacfile string) (tags map[string]string, err error) {
	tags = make(map[string]string)
	cmd := exec.Command("metaflac", "--export-tags-to=-", "--no-utf8-convert", flacfile)
	stdout, err := cmd.StdoutPipe()
	err = cmd.Start()
	if err != nil {
		return
	}
	// Read the tags. Convert double-quotes to singles, and lowercase the tag
	// names because some rippers use caps. (MusicBrainz)
	scanner := bufio.NewScanner(stdout)
	for scanner.Scan() {
		line := strings.Replace(scanner.Text(), `"`, `'`, -1)
		tag := strings.Split(line, "=")
		key := strings.ToLower(tag[0])
		value := tag[1]
		tags[key] = value
	}
	err = cmd.Wait()
	if err != nil {
		return tags, err
	}
	return tags, nil
}

// Decode a flac file to a wav file, and return the name of the wav file.
func decodeFlac(fn string) (string, error) {
	ofile, err := ioutil.TempFile("", "cnv")
	if err != nil {
		return "", err
	}
	ofile.Close()
	defer func() {
		if err != nil {
			os.Remove(ofile.Name())
		}
	}()
	cmd := exec.Command("flac", "-f", "-d", fn, "-o", ofile.Name())
	err = cmd.Start()
	if err != nil {
		return "", err
	}
	err = cmd.Wait()
	if err != nil {
		return "", err
	}
	return ofile.Name(), nil
}

// Encode a wav file to mp3 by calling faac.
func encodeMp3(inf, outf string, tags map[string]string) error {
	outdir := path.Dir(outf)
	err := os.MkdirAll(outdir, 0755)
	if err != nil {
		return err
	}
	requiredTags := [...]string{"artist", "title", "genre", "album",
		"tracknumber", "date"}
	for _, tag := range requiredTags {
		if len(tags[tag]) == 0 {
			return fmt.Errorf("Required tag %v missing on %v", tag, inf)
		}
	}
	cmd := exec.Command("lame", "-q", "2", "--vbr-new",
		"-b", "192", "-B", "320", "--preset", "extreme",
		"--ta", tags["artist"],
		"--tt", tags["title"],
		"--tg", tags["genre"],
		"--tl", tags["album"],
		"--tn", tags["tracknumber"],
		"--ty", tags["date"],
		inf, outf)
	err = cmd.Start()
	if err != nil {
		return err
	}
	err = cmd.Wait()
	if err != nil {
		return err
	}
	return nil
}
