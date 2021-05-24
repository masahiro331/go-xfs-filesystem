package main

import (
	"bufio"
	"fmt"
	"log"
	"os"
	"strings"

	"github.com/masahiro331/go-xfs-filesystem/xfs"
	"golang.org/x/xerrors"
)

const (
	INODE  = "inode"
	LS     = "ls"
	PRINT  = "print"
	CD     = "cd"
	DEBUG  = "debug"
	TREE   = "tree"
	SEARCH = "search"
)

func main() {
	args := os.Args
	if len(args) < 2 {
		log.Fatalf("invalid arguments error")
	}

	filename := args[1]

	if err := run(filename); err != nil {
		log.Fatalf("%+v\n", err)
	}
}

var (
	Stdin  = *os.Stdin
	Stdout = *os.Stdout
)

var sc = bufio.NewScanner(&Stdin)

func run(filename string) error {
	f, err := os.Open(filename)
	if err != nil {
		log.Fatalf("failed to open error %+v", err)
		return xerrors.Errorf("failed to open file: %w", err)
	}

	fs, err := xfs.NewFileSystem(f)
	if err != nil {
		return xerrors.Errorf("failed to new file system: %w", err)
	}

	for {
		var s string
		var err error
		fmt.Print("> ")
		if sc.Scan() {
			commands := strings.Fields(sc.Text())
			if len(commands) == 0 {
				continue
			}
			switch commands[0] {
			case CD:
				s, err = fs.ChangeDirectory(commands...)
				if err != nil {
					err = xerrors.Errorf("cd: %s", err)
				}
			case PRINT:
				s, err = fs.Print(commands...)
				if err != nil {
					err = xerrors.Errorf("print: %s", err)
				}
			case LS:
				s, err = fs.ListSegments(commands...)
				if err != nil {
					err = xerrors.Errorf("ls: %s", err)
				}
			case INODE:
				s, err = fs.ChangeInode(commands...)
				if err != nil {
					err = xerrors.Errorf("inode: %s", err)
				}
			case TREE:
				s, err = fs.Tree(commands...)
			case SEARCH:
				s, err = fs.Search(commands...)
				if err != nil {
					err = xerrors.Errorf("search: %s", err)
				}
			case DEBUG:
				fs.Debug(commands...)
			default:
				fmt.Printf("invalid command %q\n", commands[0])
			}
		}
		if err != nil {
			fmt.Println(err)
		}
		fmt.Println(s)
	}
}

type AllocRec struct {
	StartBlock uint32
	BlockCount uint32
}
