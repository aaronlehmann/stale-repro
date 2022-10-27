package main

import (
	"context"
	"fmt"
	"math/rand"
	"os"
	"strconv"
	"time"

	"github.com/moby/buildkit/client"
	_ "github.com/moby/buildkit/client/connhelper/dockercontainer"
	"github.com/moby/buildkit/client/llb"
	"github.com/moby/buildkit/util/appdefaults"
	"github.com/moby/buildkit/util/progress/progresswriter"
	"golang.org/x/sync/errgroup"
)

func main() {
	rand.Seed(time.Now().Unix())

	addr := os.Getenv("BUILDKIT_HOST")
	if addr == "" {
		addr = appdefaults.Address
	}

	go func() {
		if err := pushAndRun(context.Background(), addr); err != nil {
			fmt.Fprintf(os.Stderr, "err: %s\n", err)
			os.Exit(1)
		}
	}()

	time.Sleep(10 * time.Second)
	if err := pushAndRun(context.Background(), addr); err != nil {
		fmt.Fprintf(os.Stderr, "err: %s\n", err)
		os.Exit(1)
	}
}

const imageName = "localhost:5000/alehmann/dummy:test"

func pushAndRun(ctx context.Context, addr string) error {
	if err := push(ctx, addr); err != nil {
		return err
	}
	return run(ctx, addr)
}

func push(ctx context.Context, addr string) error {
	st := llb.Image("alpine", llb.LinuxAmd64)
	st = st.Run(
		llb.Shlex(`mkdir /files && touch /files/` + strconv.Itoa(rand.Int())),
	).Root()

	def, err := st.Marshal(ctx)
	if err != nil {
		return err
	}

	c, err := client.New(ctx, addr, client.WithFailFast())
	if err != nil {
		return err
	}

	pw, err := progresswriter.NewPrinter(context.TODO(), os.Stderr, "plain")
	if err != nil {
		return err
	}
	mw := progresswriter.NewMultiWriter(pw)

	eg, ctx := errgroup.WithContext(ctx)

	eg.Go(func() error {
		solveOpt := client.SolveOpt{
			Exports: []client.ExportEntry{{
				Type: client.ExporterImage,
				Attrs: map[string]string{
					"name": imageName,
					"push": "true",
				},
			}},
		}
		_, err := c.Solve(ctx, def, solveOpt, progresswriter.ResetTime(mw.WithPrefix("", false)).Status())
		return err
	})

	eg.Go(func() error {
		<-pw.Done()
		return pw.Err()
	})

	return eg.Wait()
}

func run(ctx context.Context, addr string) error {
	st := llb.Image(imageName, llb.LinuxAmd64, llb.IgnoreCache)
	st = st.Run(
		llb.Shlex(`find /files`),
		llb.IgnoreCache,
	).Root()
	st = st.Run(
		llb.Shlex(`sleep infinity`),
		llb.IgnoreCache,
	).Root()

	def, err := st.Marshal(ctx)
	if err != nil {
		return err
	}

	c, err := client.New(ctx, addr, client.WithFailFast())
	if err != nil {
		return err
	}

	pw, err := progresswriter.NewPrinter(context.TODO(), os.Stderr, "plain")
	if err != nil {
		return err
	}
	mw := progresswriter.NewMultiWriter(pw)

	eg, ctx := errgroup.WithContext(ctx)

	eg.Go(func() error {
		solveOpt := client.SolveOpt{}

		_, err := c.Solve(ctx, def, solveOpt, progresswriter.ResetTime(mw.WithPrefix("", false)).Status())
		return err
	})

	eg.Go(func() error {
		<-pw.Done()
		return pw.Err()
	})

	return eg.Wait()
}
