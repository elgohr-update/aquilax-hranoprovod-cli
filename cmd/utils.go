package cmd

import (
	"io"
	"os"

	"github.com/aquilax/hranoprovod-cli/v2/options"
	"github.com/urfave/cli/v2"
)

type cmdUtils struct {
	withFileReaders func(fileNames []string, cb func([]io.Reader) error) error
	withOptions     func(c *cli.Context, cb func(*options.Options) error) error
}

func NewCmdUtils() cmdUtils {
	return cmdUtils{
		withFileReaders: func(fileNames []string, cb func([]io.Reader) error) error {
			result := make([]io.Reader, len(fileNames))
			for i, fileName := range fileNames {
				if f, err := os.Open(fileName); err != nil {
					return err
				} else {
					defer f.Close()
					result[i] = f
				}
			}
			return cb(result)
		},
		withOptions: func(c *cli.Context, cb func(*options.Options) error) error {
			o := options.New()
			if err := o.Load(c); err != nil {
				return err
			}
			return cb(o)
		},
	}
}
