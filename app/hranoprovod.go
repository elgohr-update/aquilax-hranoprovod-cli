package app

import (
	"fmt"
	"io"
	"sort"
	"time"

	"github.com/aquilax/hranoprovod-cli/v2/filter"
	"github.com/aquilax/hranoprovod-cli/v2/parser"
	"github.com/aquilax/hranoprovod-cli/v2/reporter"
	"github.com/aquilax/hranoprovod-cli/v2/resolver"
	"github.com/aquilax/hranoprovod-cli/v2/shared"
)

type RegisterConfig struct {
	DateFormat     string
	ParserConfig   parser.Config
	ResolverConfig resolver.Config
	ReporterConfig reporter.Config
	FilterConfig   filter.Config
}

// Register generates report
func Register(logStream, dbStream io.Reader, rc RegisterConfig) error {
	rpCb := func(rpc reporter.Config, nl shared.DBNodeList) reporter.Reporter {
		return reporter.NewRegReporter(rpc, nl)
	}
	return walkWithReporter(logStream, dbStream, rc.DateFormat, rc.ParserConfig, rc.ResolverConfig, rc.ReporterConfig, rc.FilterConfig, rpCb)
}

type BalanceConfig = RegisterConfig

// Balance generates balance report
func Balance(logStream, dbStream io.Reader, bc BalanceConfig) error {
	return withResolvedDatabase(dbStream, bc.ParserConfig, bc.ResolverConfig,
		func(nl shared.DBNodeList) error {
			r := reporter.NewBalanceReporter(bc.ReporterConfig, nl)
			return walkNodesInStream(logStream, bc.DateFormat, bc.ParserConfig, getIntervalNodeFilter(bc.FilterConfig), r)
		})
}

type ReportUnresolvedConfig = RegisterConfig

// ReportUnresolved generates report for unresolved elements
func ReportUnresolved(logStream, dbStream io.Reader, ruc ReportUnresolvedConfig) error {
	return withResolvedDatabase(dbStream, ruc.ParserConfig, ruc.ResolverConfig,
		func(nl shared.DBNodeList) error {
			r := reporter.NewUnsolvedReporter(ruc.ReporterConfig, nl)
			return walkNodesInStream(logStream, ruc.DateFormat, ruc.ParserConfig, getIntervalNodeFilter(ruc.FilterConfig), r)
		})
}

type SummaryConfig = RegisterConfig

// Summary generates summary
func Summary(logStream, dbStream io.Reader, sc SummaryConfig) error {
	return withResolvedDatabase(dbStream, sc.ParserConfig, sc.ResolverConfig,
		func(nl shared.DBNodeList) error {
			r := reporter.NewSummaryReporterTemplate(sc.ReporterConfig, nl)
			return walkNodesInStream(logStream, sc.DateFormat, sc.ParserConfig, getIntervalNodeFilter(sc.FilterConfig), r)
		})
}

type PrintConfig struct {
	DateFormat     string
	ParserConfig   parser.Config
	ReporterConfig reporter.Config
	FilterConfig   filter.Config
}

// Print reads and prints back out the log file
func Print(logStream io.Reader, pc PrintConfig) error {
	r := reporter.NewPrintReporter(pc.ReporterConfig)
	return walkNodesInStream(logStream, pc.DateFormat, pc.ParserConfig, getIntervalNodeFilter(pc.FilterConfig), r)
}

type ReportQuantityConfig struct {
	DateFormat     string
	Descending     bool
	ParserConfig   parser.Config
	ReporterConfig reporter.Config
	FilterConfig   filter.Config
}

// ReportQuantity Generates a quantity report
func ReportQuantity(logStream io.Reader, rqc ReportQuantityConfig) error {
	r := reporter.NewQuantityReporter(rqc.ReporterConfig, rqc.Descending)
	return walkNodesInStream(logStream, rqc.DateFormat, rqc.ParserConfig, getIntervalNodeFilter(rqc.FilterConfig), r)
}

type CSVLogConfig struct {
	ParserConfig   parser.Config
	FilterConfig   filter.Config
	ReporterConfig reporter.CSVConfig
}

// CSVLog generates CSV export of the log
func CSVLog(logStream io.Reader, c CSVLogConfig) error {
	r := reporter.NewCSVReporter(c.ReporterConfig)
	return walkNodesInStream(logStream, c.ParserConfig.DateFormat, c.ParserConfig, getIntervalNodeFilter(c.FilterConfig), r)
}

type CSVDatabaseConfig struct {
	ParserConfig   parser.Config
	ReporterConfig reporter.Config
}

// CSVDatabase generates CSV export of the database
func CSVDatabase(dbStream io.Reader, cdc CSVDatabaseConfig) error {
	p := parser.NewParser(cdc.ParserConfig)
	r := reporter.NewCSVDatabaseReporter(cdc.ReporterConfig)
	go p.ParseStream(dbStream)
	return func() error {
		for {
			select {
			case node := <-p.Nodes:
				r.Process(shared.NewDBNodeFromNode(node))
			case error := <-p.Errors:
				return error
			case <-p.Done:
				return r.Flush()
			}
		}
	}()
}

type CSVDatabaseResolvedConfig struct {
	ParserConfig   parser.Config
	ReporterConfig reporter.Config
	ResolverConfig resolver.Config
}

// CSVDatabaseResolved generates CSV export of the resolved database
func CSVDatabaseResolved(dbStream io.Reader, cdrc CSVDatabaseResolvedConfig) error {
	nl, err := loadDatabaseFromStream(dbStream, cdrc.ParserConfig)
	if err != nil {
		return err
	}
	nl, err = resolver.Resolve(cdrc.ResolverConfig, nl)
	if err != nil {
		return err
	}
	r := reporter.NewCSVDatabaseReporter(cdrc.ReporterConfig)
	for _, n := range nl {
		if err = r.Process(n); err != nil {
			return err
		}
	}
	return r.Flush()
}

type LintConfig struct {
	Silent         bool
	ParserConfig   parser.Config
	ReporterConfig reporter.Config
}

// Lint lints file
func Lint(stream io.Reader, lc LintConfig) error {
	parser := parser.NewParser(lc.ParserConfig)
	go parser.ParseStream(stream)
	err := func() error {
		for {
			select {
			case <-parser.Nodes:
			case err := <-parser.Errors:
				fmt.Fprintln(lc.ReporterConfig.Output, err)
			case <-parser.Done:
				return nil
			}
		}
	}()
	if err != nil {
		return err
	}
	if !lc.Silent {
		fmt.Fprintln(lc.ReporterConfig.Output, "No errors found")
	}
	return nil
}

type ReportElementConfig struct {
	ElementName    string
	Descending     bool
	ParserConfig   parser.Config
	ResolverConfig resolver.Config
	ReporterConfig reporter.Config
}

// ReportElement generates report for single element
func ReportElement(dbStream io.Reader, rec ReportElementConfig) error {
	nl, err := loadDatabaseFromStream(dbStream, rec.ParserConfig)
	if err != nil {
		return err
	}
	nl, err = resolver.Resolve(rec.ResolverConfig, nl)
	if err != nil {
		return err
	}
	var list []shared.Element
	for name, node := range nl {
		for _, el := range node.Elements {
			if el.Name == rec.ElementName {
				list = append(list, shared.NewElement(name, el.Value))
			}
		}
	}
	if rec.Descending {
		sort.SliceStable(list, func(i, j int) bool {
			return list[i].Value > list[j].Value
		})
	} else {
		sort.SliceStable(list, func(i, j int) bool {
			return list[i].Value < list[j].Value
		})
	}
	return reporter.NewElementReporter(rec.ReporterConfig, list).Flush()
}

type StatsConfig struct {
	ParserConfig   parser.Config
	ReporterConfig reporter.Config
}

// Stats generates statistics report
func Stats(logFileName, dbFileName string, sc StatsConfig) error {
	var err error
	var firstLogDate time.Time
	var lastLogDate time.Time

	countLog := 0
	if err = parser.ParseFileCallback(logFileName, sc.ParserConfig, func(n *shared.ParserNode, _ error) (stop bool, cbError error) {
		lastLogDate, err = time.Parse(sc.ReporterConfig.DateFormat, n.Header)
		if err == nil {
			if firstLogDate.IsZero() {
				firstLogDate = lastLogDate
			}
		}
		countLog++
		return false, nil
	}); err != nil {
		return err
	}

	countDb := 0
	if err = parser.ParseFileCallback(dbFileName, sc.ParserConfig, func(n *shared.ParserNode, _ error) (stop bool, cbError error) {
		countDb++
		return false, nil
	}); err != nil {
		return err
	}

	return reporter.NewStatsReporter(sc.ReporterConfig, &reporter.Stats{
		DbFileName:      dbFileName,
		LogFileName:     logFileName,
		DbRecordsCount:  countDb,
		LogRecordsCount: countLog,
		LogFirstRecord:  firstLogDate,
		LogLastRecord:   lastLogDate,
	}).Flush()
}

type resolvedCallback = func(nl shared.DBNodeList) error

func withResolvedDatabase(dbStream io.Reader, pc parser.Config, rc resolver.Config, cb resolvedCallback) error {
	if nl, err := loadDatabaseFromStream(dbStream, pc); err == nil {
		if nl, err = resolver.Resolve(rc, nl); err == nil {
			return cb(nl)
		} else {
			return err
		}
	} else {
		return err
	}
}

type reporterCallback func(rpc reporter.Config, nl shared.DBNodeList) reporter.Reporter

func walkWithReporter(logStream, dbStream io.Reader, dateFormat string, pc parser.Config, rc resolver.Config, rpc reporter.Config, fc filter.Config, rpCb reporterCallback) error {
	return withResolvedDatabase(dbStream, pc, rc,
		func(nl shared.DBNodeList) error {
			r := rpCb(rpc, nl)
			return walkNodesInStream(logStream, dateFormat, pc, getIntervalNodeFilter(fc), r)
		})
}

func loadDatabaseFromStream(dbStream io.Reader, pc parser.Config) (shared.DBNodeList, error) {
	nodeList := shared.NewDBNodeList()
	return nodeList, parser.ParseStreamCallback(dbStream, pc, func(node *shared.ParserNode, err error) (stop bool, cbError error) {
		if err != nil {
			return true, err
		} else {
			nodeList.Push(shared.NewDBNodeFromNode(node))
			return false, nil
		}
	})
}

type LogNodeFilter = func(t time.Time, node *shared.ParserNode) (bool, error)

func getIntervalNodeFilter(fc filter.Config) *LogNodeFilter {
	if fc.BeginningTime == nil && fc.EndTime == nil {
		// no filter if beginning and end time are nil
		return nil
	}

	inInterval := func(t time.Time) bool {
		if (fc.BeginningTime != nil && !isGoodDate(t, *fc.BeginningTime, dateBeginning)) || (fc.EndTime != nil && !isGoodDate(t, *fc.EndTime, dateEnd)) {
			return false
		}
		return true
	}

	filter := func(t time.Time, node *shared.ParserNode) (bool, error) {
		return inInterval(t), nil
	}
	return &filter
}

func walkNodesInStream(logStream io.Reader, dateFormat string, pc parser.Config, filter *LogNodeFilter, r reporter.Reporter) error {
	var ln *shared.LogNode
	var t time.Time
	var ok bool

	cb := func(node *shared.ParserNode, err error) (stop bool, cbError error) {
		if err != nil {
			return true, err
		}
		if t, err = time.Parse(dateFormat, node.Header); err != nil {
			return true, err
		}
		ok = true
		if filter != nil {
			if ok, err = (*filter)(t, node); err != nil {
				return true, err
			}
		}
		if ok {
			if ln, err = shared.NewLogNodeFromElements(t, node.Elements, node.Metadata); err != nil {
				return true, err
			}
			if err = r.Process(ln); err != nil {
				return true, err
			}
		}
		return false, nil
	}
	err := parser.ParseStreamCallback(logStream, pc, cb)
	if err != nil {
		return err
	}
	return r.Flush()
}

// compareType identifies the type of date comparison
type compareType bool

const (
	dateBeginning compareType = true
	dateEnd       compareType = false
)

func isGoodDate(time, compareTime time.Time, ct compareType) bool {
	if time.Equal(compareTime) {
		return true
	}
	if ct == dateBeginning {
		return time.After(compareTime)
	}
	return time.Before(compareTime)
}
