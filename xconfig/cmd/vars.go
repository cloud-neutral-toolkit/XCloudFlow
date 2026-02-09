// cmd/vars.go
package cmd

var (
	AggregateOutput bool   // --aggregate / -A
	CheckMode       bool   // --check / -C
	DiffMode        bool   // --diff / -D
	InventoryPath   string // --inventory / -i
	MaxWorkers      int    // --forks / -f
	DSN             string // --dsn (defaults to DATABASE_URL)
)
