package insight

import (
	"encoding/json"
	"errors"
	"net/http"
	"strconv"

	"github.com/boltdb/bolt"
	"github.com/go-kit/kit/metrics"
	"go.uber.org/zap"
)

// Server stores counters
type Server struct {
	Log     *zap.Logger
	Counter metrics.Counter
	Db      *bolt.DB
}

// Handler for monitoring actions
func Handler(s Server) func(w http.ResponseWriter, r *http.Request) {
	return func(w http.ResponseWriter, r *http.Request) {
		s.Log.Debug("started handling")
		req, err := decodeHTTPRequest(r)
		if err != nil {
			s.Log.Error("failed handling", zap.Error(err))
			http.Error(w, err.Error(), http.StatusInternalServerError)
			panic(err)
		}
		err = s.Count(req.Type, req.App)
		if err != nil {
			s.Log.Error("failed incrementing", zap.Error(err))
			http.Error(w, err.Error(), http.StatusInternalServerError)
			panic(err)
		}
		w.WriteHeader(http.StatusOK)
		s.Log.Info("finished handling", zap.String("type", req.Type), zap.String("app", req.App))
	}
}

// Count increments the db and prom counter
func (s Server) Count(ctype, app string) error {
	s.Counter.With("type", ctype, "app", app).Add(1)
	return s.Db.Update(func(tx *bolt.Tx) (err error) {
		b := tx.Bucket([]byte(app))
		if b == nil {
			b, err = tx.CreateBucket([]byte(app))
			if err != nil {
				s.Log.Error("db bucketCreate error",
					zap.String("type", ctype),
					zap.String("app", app),
					zap.Error(err),
				)
				return err
			}
			err = b.Put([]byte(ctype), []byte("0"))
			if err != nil {
				s.Log.Error("db put error",
					zap.String("type", ctype),
					zap.String("app", app),
					zap.Error(err),
				)
				return err
			}
		}
		v := b.Get([]byte(ctype))
		if len(v) < 1 {
			v = []byte("0")
		}
		vi, err := strconv.Atoi(string(v))
		if err != nil {
			s.Log.Error("count error", zap.Error(err))
			return err
		}
		vi = vi + 1
		v = []byte(strconv.Itoa(vi))
		err = b.Put([]byte(ctype), v)
		if err != nil {
			s.Log.Error("db put error",
				zap.String("type", ctype),
				zap.String("app", app),
				zap.Error(err),
			)
			return err
		}
		return nil
	})
}

// Request defines a default request
type Request struct {
	Type string `json:"type"`
	App  string `json:"app"`
}

func decodeHTTPRequest(r *http.Request) (Request, error) {
	var req Request
	err := json.NewDecoder(r.Body).Decode(&req)
	if err != nil {
		return req, err
	}
	if req.App == "" {
		return req, errors.New("missing key: app")
	}
	if req.Type == "" {
		return req, errors.New("missing key: type")
	}
	return req, err
}
