# cast

The easiest command to watch your contents on TV.

## Getting Started

### Installing

You can install it via `go get`.

```console
$ go isntall github.com/ichiban/cast/cmd/cast@latest
```

### Usage

After installation, you can simply type `cast` command and a UPnP Media Server named "Cast" will appear on media players like VLC under Local Network section.
The "Cast" Media Server contains every media file in your current directory.

```console
$ cast
```

You can halt the command by pressing `Ctrl+C`.

Alternatively, you can specify the directory:

```console
$ cast -dir ~/Movies
```

Other options can be found in `cast -h`.