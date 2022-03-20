package reporter

import (
	"bufio"
	"fmt"
	"time"

	"github.com/aquilax/hranoprovod-cli/v2"
	"github.com/aquilax/hranoprovod-cli/v2/accumulator"
)

// singleReporter outputs report for single food
type singleReporter struct {
	config Config
	db     hranoprovod.DBNodeMap
	output *bufio.Writer
}

func newSingleReporter(config Config, db hranoprovod.DBNodeMap) *singleReporter {
	return &singleReporter{
		config,
		db,
		bufio.NewWriter(config.Output),
	}
}

func (r *singleReporter) Process(ln *hranoprovod.LogNode) error {
	acc := accumulator.NewAccumulator()
	singleElement := r.config.SingleElement
	for _, e := range ln.Elements {
		repl, found := r.db[e.Name]
		if found {
			for _, repl := range repl.Elements {
				if repl.Name == singleElement {
					acc.Add(repl.Name, repl.Value*e.Value)
				}
			}
		} else {
			if e.Name == singleElement {
				acc.Add(e.Name, e.Value)
			}
		}
	}
	if len(acc) > 0 {
		arr := (acc)[singleElement]
		r.printSingleElementRow(ln.Time, r.config.SingleElement, arr[accumulator.Positive], arr[accumulator.Negative])
	}
	return nil

}

func (r *singleReporter) Flush() error {
	return r.output.Flush()
}

func (r *singleReporter) printSingleElementRow(ts time.Time, name string, pos float64, neg float64) {
	format := "%s %20s %10.2f %10.2f =%10.2f\n"
	if r.config.CSV {
		format = "%s;\"%s\";%0.2f;%0.2f;%0.2f\n"
	}
	fmt.Fprintf(r.output, format, ts.Format(r.config.DateFormat), name, pos, -1*neg, pos+neg)
}
