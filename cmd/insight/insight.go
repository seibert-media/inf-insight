package main

import (
	"flag"
	"fmt"
	"net/http"
	"os"
	"runtime"
	"strconv"
	"time"

	"github.com/boltdb/bolt"
	"github.com/go-kit/kit/metrics"
	"github.com/go-kit/kit/metrics/prometheus"
	"github.com/seibert-media/inf-insight/pkg/insight"

	raven "github.com/getsentry/raven-go"
	"github.com/golang/glog"
	"github.com/kolide/kit/version"
	stdprometheus "github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
)

const (
	app    = "insight"
	appKey = "insight"
)

var (
	maxprocsPtr = flag.Int("maxprocs", runtime.NumCPU(), "max go procs")
	sentryDsn   = flag.String("sentrydsn", "https://50fe350cf0ad493d9c1c80332dd754d0:30e312ce5ed94842a2c64ccb81b75967@sentry.io/266406", "sentry dsn key")
	dbgPtr      = flag.Bool("debug", false, "debug printing")
	versionPtr  = flag.Bool("version", true, "show or hide version info")
	httpAddr    = flag.String("http.addr", ":8080", "HTTP listen address")

	sentry *raven.Client
)

func main() {
	flag.Parse()

	if *versionPtr {
		fmt.Printf("-- //S/M %s --\n", app)
		version.PrintFull()
	}
	runtime.GOMAXPROCS(*maxprocsPtr)

	// prepare glog
	defer glog.Flush()
	glog.CopyStandardLogTo("info")

	var zapFields []zapcore.Field
	// hide app and version information when debugging
	if !*dbgPtr {
		zapFields = []zapcore.Field{
			zap.String("app", appKey),
			zap.String("version", version.Version().Version),
		}
	}

	// prepare zap logging
	log := newLogger(*dbgPtr).With(zapFields...)
	defer log.Sync()
	log.Info("preparing")

	var err error

	// prepare sentry error logging
	sentry, err = raven.New(*sentryDsn)
	if err != nil {
		panic(err)
	}
	err = raven.SetDSN(*sentryDsn)
	if err != nil {
		panic(err)
	}

	// run main code
	log.Info("starting")
	defer log.Info("finished")
	raven.CapturePanicAndWait(func() {
		if err := do(log); err != nil {
			log.Fatal("fatal error encountered", zap.Error(err))
			raven.CaptureErrorAndWait(err, map[string]string{"isFinal": "true"})
		}
	}, nil)
}

func do(log *zap.Logger) error {
	log.Info("opening db", zap.String("file", "bolt.db"))
	db, err := bolt.Open("bolt.db", 0600, nil)
	if err != nil {
		log.Fatal("db open error", zap.Error(err))
	}
	defer db.Close()

	var counter metrics.Counter
	{
		counter = prometheus.NewCounterFrom(stdprometheus.CounterOpts{
			Namespace: "infinity",
			Subsystem: "insight",
			Name:      "calls_sum",
			Help:      "Total count of calls",
		}, []string{"type", "app"})
	}

	loadPreviousMetrics(log, db, counter)

	s := insight.Server{
		Log:     log,
		Counter: counter,
		Db:      db,
	}

	http.Handle("/metrics", promhttp.Handler())
	http.Handle("/add", insight.Handler(s))

	return http.ListenAndServe(*httpAddr, nil)
}

func loadPreviousMetrics(log *zap.Logger, db *bolt.DB, counter metrics.Counter) {
	db.View(func(tx *bolt.Tx) error {
		log.Info("loading previous metrics")
		defer log.Info("loaded previous metrics", zap.Duration("took", time.Since(time.Now())))
		err := tx.ForEach(func(name []byte, b *bolt.Bucket) error {
			err := b.ForEach(func(k, v []byte) error {
				vi, err := strconv.Atoi(string(v))
				if err != nil {
					log.Fatal("conversion error",
						zap.ByteString("app", name),
						zap.ByteString("type", k),
						zap.Error(err),
					)
				}
				counter.With("type", string(k), "app", string(name)).Add(float64(vi))
				return nil
			})
			return err
		})
		if err != nil {
			log.Fatal("db error", zap.Error(err))
		}
		return err
	})
}

func newLogger(dbg bool) *zap.Logger {
	highPriority := zap.LevelEnablerFunc(func(lvl zapcore.Level) bool {
		return lvl >= zapcore.ErrorLevel
	})
	lowPriority := zap.LevelEnablerFunc(func(lvl zapcore.Level) bool {
		return lvl < zapcore.ErrorLevel
	})

	consoleDebugging := zapcore.Lock(os.Stdout)
	consoleErrors := zapcore.Lock(os.Stderr)
	consoleConfig := zap.NewDevelopmentEncoderConfig()
	consoleEncoder := zapcore.NewConsoleEncoder(consoleConfig)
	core := zapcore.NewTee(
		zapcore.NewCore(consoleEncoder, consoleErrors, highPriority),
		zapcore.NewCore(consoleEncoder, consoleDebugging, lowPriority),
	)
	logger := zap.New(core)
	if dbg {
		logger = logger.WithOptions(
			zap.AddCaller(),
			zap.AddStacktrace(zap.ErrorLevel),
		)
	} else {
		logger = logger.WithOptions(
			zap.AddStacktrace(zap.FatalLevel),
		)
	}
	return logger
}
