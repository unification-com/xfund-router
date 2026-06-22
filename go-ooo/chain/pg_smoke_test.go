package chain

import (
	"os"
	"testing"

	"go-ooo/database"
	"go-ooo/database/models"

	"github.com/prometheus/client_golang/prometheus/testutil"
	"github.com/stretchr/testify/require"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
)

// TestPostgresWarmStartSmoke validates the fulfilment count methods and the metrics warm-start
// against a real Postgres database (the production dialect - the unit tests run on sqlite).
// Skipped unless GOOOO_TEST_PG_DSN is set, e.g.
//
//	GOOOO_TEST_PG_DSN="host=/tmp port=55432 user=postgres dbname=ooo_sepolia sslmode=disable" \
//	  go test ./chain/ -run TestPostgresWarmStartSmoke -v
func TestPostgresWarmStartSmoke(t *testing.T) {
	dsn := os.Getenv("GOOOO_TEST_PG_DSN")
	if dsn == "" {
		t.Skip("set GOOOO_TEST_PG_DSN to run the Postgres warm-start smoke test")
	}

	gdb, err := gorm.Open(postgres.Open(dsn), &gorm.Config{})
	require.NoError(t, err)
	d := &database.DB{DB: gdb}

	// Pick a chain id present in the dump - the counts + warm-start are per-chain.
	var chainIds []int64
	require.NoError(t, d.Model(&models.DataRequests{}).Distinct().Pluck("chain_id", &chainIds).Error)
	require.NotEmpty(t, chainIds, "the dump has at least one request")
	chainId := chainIds[0]
	label := metricChainLabel(chainId)

	sent, err := d.CountFulfilmentsSent(chainId)
	require.NoError(t, err)
	success, err := d.CountRequestsByStatus(chainId, models.REQUEST_STATUS_SUCCESS)
	require.NoError(t, err)
	reverted, err := d.CountFailedFulfilments(chainId)
	require.NoError(t, err)
	t.Logf("postgres fulfilment history (chain %d): sent=%d success=%d reverted=%d", chainId, sent, success, reverted)

	// Invariants that hold for any real database.
	require.GreaterOrEqual(t, sent, success, "every success was broadcast at least once")
	require.GreaterOrEqual(t, sent, int64(0))
	require.GreaterOrEqual(t, reverted, int64(0))

	// The warm-start must seed each per-chain counter with exactly the count it read from the DB.
	sentBefore := testutil.ToFloat64(fulfilmentSentTotal.WithLabelValues(label))
	successBefore := testutil.ToFloat64(fulfilmentResultTotal.WithLabelValues(label, "success"))
	revertedBefore := testutil.ToFloat64(fulfilmentResultTotal.WithLabelValues(label, "reverted"))

	WarmStartFulfilmentMetrics(d, []int64{chainId})

	require.EqualValues(t, sent, testutil.ToFloat64(fulfilmentSentTotal.WithLabelValues(label))-sentBefore, "sent seeded")
	require.EqualValues(t, success, testutil.ToFloat64(fulfilmentResultTotal.WithLabelValues(label, "success"))-successBefore, "success seeded")
	require.EqualValues(t, reverted, testutil.ToFloat64(fulfilmentResultTotal.WithLabelValues(label, "reverted"))-revertedBefore, "reverted seeded")
}
