# cast

The easiest command to watch your contents on TV.

## Getting Started

### Installing

You can install it via `go get`.

```console
$ go get github.com/ichiban/cast/cmd/cast
```

### Usage

After installation, you can simply type `cast` command and a UPnP Media Server named "Cast" will appear on media players like VLC under Local Network section.
The "Cast" Media Server contains every media file in your current directory.

```console
$ cast
```

You can halt the command by pressing `Ctrl+C`.

Alternatively, you can specify directories or files to expose by passing path arguments.
The example above can be written with `.` argument:

```console
$ cast .
```

You can also expose individual file(s):

```console
$ cast a.mp4 b.mp4 c.mp4
```

Or multiple directories:

```console
$ cast ~/Pictures ~/Music ~/Movies
```

Options can be found in `cast -h`.