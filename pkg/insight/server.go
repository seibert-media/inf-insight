package insight

import (
	"encoding/json"
	"fmt"
	"log"
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
			fmt.Fprint(w, err.Error())
			w.WriteHeader(http.StatusBadRequest)
		}
		s.Count(req.Type, req.App)
		s.Log.Info("finished handling", zap.String("type", req.Type), zap.String("app", req.App))
	}
}

// Count increments the db and prom counter
func (s Server) Count(ctype, app string) {
	s.Counter.With("type", ctype, "app", app).Add(1)
	s.Db.Update(func(tx *bolt.Tx) (err error) {
		b := tx.Bucket([]byte(app))
		if b == nil {
			b, err = tx.CreateBucket([]byte(app))
			if err != nil {
				log.Fatal("db bucketCreate error",
					zap.String("type", ctype),
					zap.String("app", app),
					zap.Error(err),
				)
			}
			err = b.Put([]byte(ctype), []byte("0"))
			if err != nil {
				log.Fatal("db put error",
					zap.String("type", ctype),
					zap.String("app", app),
					zap.Error(err),
				)
			}
		}
		v := b.Get([]byte(ctype))
		if len(v) < 1 {
			v = []byte("0")
		}
		vi, err := strconv.Atoi(string(v))
		if err != nil {
			log.Fatal("count error", zap.Error(err))
		}
		vi = vi + 1
		v = []byte(strconv.Itoa(vi))
		err = b.Put([]byte(ctype), v)
		if err != nil {
			log.Fatal("db put error",
				zap.String("type", ctype),
				zap.String("app", app),
				zap.Error(err),
			)
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
	return req, err
}
