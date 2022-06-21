package main

import (
	"flag"
	"fmt"
	"log"
	"os"
	"sort"
	"strings"

	"github.com/gocarina/gocsv"
	dto "github.com/prometheus/client_model/go"
	"github.com/prometheus/common/expfmt"
)

type Table struct {
	Prefix  string                       `csv:"prefix"`
	Title   string                       `csv:"title"`
	Metrics map[string]*dto.MetricFamily `csv:"-"`
}

func fatal(err error) {
	if err != nil {
		log.Fatalln(err)
	}
}

func parseMetricFamilies(path string) (map[string]*dto.MetricFamily, error) {
	reader, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer reader.Close()

	var parser expfmt.TextParser
	mf, err := parser.TextToMetricFamilies(reader)
	if err != nil {
		return nil, err
	}
	return mf, nil
}

func parseCSV(path string) ([]*Table, error) {
	tablesFile, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer tablesFile.Close()

	tables := []*Table{}
	err = gocsv.UnmarshalFile(tablesFile, &tables)
	return tables, err
}

func main() {
	f := flag.String("f", "", "set output filepath")
	c := flag.String("c", "", "set csv config filepath")
	flag.Parse()

	metricFamilies, err := parseMetricFamilies(*f)
	fatal(err)

	tables, err := parseCSV(*c)
	fatal(err)

	tableMap := make(map[string]Table)
	tablePrefixes := make([]string, 0)
	for _, table := range tables {
		table.Metrics = make(map[string]*dto.MetricFamily)
		tableMap[table.Prefix] = *table
		tablePrefixes = append(tablePrefixes, table.Prefix)
	}
	tablePrefixes = append(tablePrefixes, "__default_table__")
	tableMap["__default_table__"] = Table{
		Prefix:  "__default_table__",
		Title:   "Other Metrics",
		Metrics: make(map[string]*dto.MetricFamily),
	}

	for mfKey := range metricFamilies {
		foundPrefix := false
		for _, tablePrefix := range tablePrefixes {
			if strings.HasPrefix(mfKey, tablePrefix) {
				tableMap[tablePrefix].Metrics[mfKey] = metricFamilies[mfKey]
				foundPrefix = true
				break
			}
		}
		if !foundPrefix {
			tableMap["__default_table__"].Metrics[mfKey] = metricFamilies[mfKey]
		}
	}
	fmt.Printf(`
////
THIS CONTENT IS GENERATED FROM THE FOLLOWING FILES:
- Prometheus metrics file path %s
- Tables config file path %s
Each row in the tables config file results in a new metrics table
where the 1st column is the metrics name prefix to match on (e.g. go_)
and the 2nd column is the table title (e.g. "Go Runtime Metrics")
////
`, *f, *c)

	for tpi := range tablePrefixes {
		tablePrefix := tablePrefixes[tpi]
		table := tableMap[tablePrefix]
		if len(table.Metrics) > 0 {
			fmt.Printf(".%s\n", table.Title)
			fmt.Println("|===")
			fmt.Println("|Name |Help |Type |Labels")

			keys := make([]string, 0, len(table.Metrics))
			for k := range table.Metrics {
				keys = append(keys, k)
			}
			sort.Strings(keys)

			for _, key := range keys {
				fmt.Printf("| `%s` ", table.Metrics[key].GetName())
				fmt.Printf("| %s", table.Metrics[key].GetHelp())
				fmt.Printf("| %s", table.Metrics[key].GetType())

				labelstring := ""
				metrics := table.Metrics[key].GetMetric()
				if len(metrics) > 0 {
					// just take the first metric in the array as it has sufficient info
					metric := metrics[0]
					labels := metric.GetLabel()
					if len(labels) > 0 {
						for _, label := range labels {
							labelstring = labelstring + "`" + label.GetName() + "` "
						}
					}
				}
				fmt.Println("|", labelstring)
			}

			fmt.Println("|===")
		}
	}
}
