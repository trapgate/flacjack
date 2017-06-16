# flacjack [![Go Report Card](https://goreportcard.com/badge/github.com/trapgate/flacjack)](https://goreportcard.com/report/github.com/trapgate/flacjack)
Convert all FLAC files in a directory tree to MP3 files

## Details
This program converts all FLAC files in a directory tree into MP3 files, which
can be output to the same directory tree or a parallel one rooted elsewhere. The
program will create all directories it needs to in the destination directory
tree. As a simple example, if you point the program at a directory containing
two subdirectories with FLAC files in them:

artist1/album1
artist2/album1

The program will create directories with the same names in the destination tree,
and place the converted MP3 files there.

The command-line programs "flac", "metaflac", and "lame" must be installed and
in the path.

The FLAC files must have tags, which will be set on the converted files too. The
required flags are: artist, title, genre, album, tracknumber, and date.

The program will run as many converters in parallel as there are cores. It
outputs to the command line and uses terminal escape codes to output in color.
Right now it doesn't bother checking to see if it's outputting to a tty before
using the codes.
