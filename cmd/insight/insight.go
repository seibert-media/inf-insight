package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"net/http"
	"os"
	"runtime"

	"github.com/go-kit/kit/metrics"
	"github.com/go-kit/kit/metrics/prometheus"

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
	sentryDsn   = flag.String("sentrydsn", "", "sentry dsn key")
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

	var started, finished, errored metrics.Counter
	{
		started = prometheus.NewCounterFrom(stdprometheus.CounterOpts{
			Namespace: "infinity",
			Subsystem: "insight",
			Name:      "start_calls_sum",
			Help:      "Total count of start calls",
		}, []string{"app"})
		finished = prometheus.NewCounterFrom(stdprometheus.CounterOpts{
			Namespace: "infinity",
			Subsystem: "insight",
			Name:      "finish_calls_sum",
			Help:      "Total count of start calls",
		}, []string{"app"})
		errored = prometheus.NewCounterFrom(stdprometheus.CounterOpts{
			Namespace: "infinity",
			Subsystem: "insight",
			Name:      "error_calls_sum",
			Help:      "Total count of start calls",
		}, []string{"app"})
	}

	s := InsightServer{
		log:      log,
		started:  started,
		finished: finished,
		errored:  errored,
	}

	http.Handle("/metrics", promhttp.Handler())
	http.Handle("/start", StartHandler(s))
	http.Handle("/finish", FinishHandler(s))
	http.Handle("/error", ErrorHandler(s))

	return http.ListenAndServe(*httpAddr, nil)
}

// InsightServer stores counters
type InsightServer struct {
	log      *zap.Logger
	started  metrics.Counter
	finished metrics.Counter
	errored  metrics.Counter
}

// InsightRequest defines a default request
type InsightRequest struct {
	App string `json:"app"`
}

// StartHandler for monitoring start actions
func StartHandler(s InsightServer) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		s.log.Info("started handling", zap.String("func", "start"))

		req, err := decodeHTTPRequest(r)
		if err != nil {
			s.log.Error("failed handling", zap.String("func", "start"), zap.Error(err))
			fmt.Fprint(w, err.Error())
			w.WriteHeader(http.StatusBadRequest)
		}
		app := req.App

		s.started.With("app", app).Add(1)
		s.log.Info("finished handling", zap.String("func", "start"), zap.String("app", app))
	})
}

// FinishHandler for monitoring finish actions
func FinishHandler(s InsightServer) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		s.log.Info("started handling", zap.String("func", "finish"))

		req, err := decodeHTTPRequest(r)
		if err != nil {
			s.log.Error("failed handling", zap.String("func", "finish"), zap.Error(err))
			fmt.Fprint(w, err.Error())
			w.WriteHeader(http.StatusBadRequest)
		}
		app := req.App

		s.finished.With("app", app).Add(1)
		s.log.Info("finished handling", zap.String("func", "finish"), zap.String("app", app))
	})
}

// ErrorHandler for monitoring error actions
func ErrorHandler(s InsightServer) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		s.log.Info("started handling", zap.String("func", "error"))

		req, err := decodeHTTPRequest(r)
		if err != nil {
			s.log.Error("failed handling", zap.String("func", "error"), zap.Error(err))
			fmt.Fprint(w, err.Error())
			w.WriteHeader(http.StatusBadRequest)
		}
		app := req.App

		s.errored.With("app", app).Add(1)
		s.log.Info("finished handling", zap.String("func", "error"), zap.String("app", app))
	})
}

func decodeHTTPRequest(r *http.Request) (InsightRequest, error) {
	var req InsightRequest
	err := json.NewDecoder(r.Body).Decode(&req)
	return req, err
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
