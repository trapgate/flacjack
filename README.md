# flacjack
Convert all FLAC files in a directory tree to MP3 files

## Details
This program converts all FLAC files in a directory tree into MP# files, which
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
