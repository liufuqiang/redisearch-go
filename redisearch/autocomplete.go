package redisearch

import (
	"strconv"

	"github.com/garyburd/redigo/redis"
)

// Autocompleter implements a redisearch auto-completer API
type Autocompleter struct {
	pool *redis.Pool
	name string
}

// NewAutocompleter creates a new Autocompleter with the given host and key name
func NewAutocompleter(addr, name string) *Autocompleter {
	return &Autocompleter{
		pool: redis.NewPool(func() (redis.Conn, error) {
			return redis.Dial("tcp", addr)
		}, maxConns),
		name: name,
	}
}

// Delete deletes the Autocompleter key for this AC
func (a *Autocompleter) Delete() error {

	conn := a.pool.Get()
	defer conn.Close()

	_, err := conn.Do("DEL", a.name)
	return err
}

// AddTerms pushes new term suggestions to the index
func (a *Autocompleter) AddTerms(withPayload bool, terms ...Suggestion) error {

	conn := a.pool.Get()
	defer conn.Close()

	i := 0
	for _, term := range terms {
		if withPayload {
			if err := conn.Send("FT.SUGADD", a.name, term.Term, term.Score, "PAYLOAD", term.Payload); err != nil {
				return err
			}
		} else {
			if err := conn.Send("FT.SUGADD", a.name, term.Term, term.Score); err != nil {
				return err
			}
		}
		i++
	}
	if err := conn.Flush(); err != nil {
		return err
	}
	for i > 0 {
		if _, err := conn.Receive(); err != nil {
			return err
		}
		i--
	}
	return nil
}

// Suggest gets completion suggestions from the Autocompleter dictionary to the given prefix.
// If fuzzy is set, we also complete for prefixes that are in 1 Levenshten distance from the
// given prefix
func (a *Autocompleter) Suggest(prefix string, num int, fuzzy bool, payload bool) ([]Suggestion, error) {
	conn := a.pool.Get()
	defer conn.Close()

	args := redis.Args{a.name, prefix, "MAX", num, "WITHSCORES"}
	if fuzzy {
		args = append(args, "FUZZY")
	}
	div := 2
	if payload {
		args = append(args, "WITHPAYLOADS")
		div = 3
	}
	vals, err := redis.Strings(conn.Do("FT.SUGGET", args...))
	if err != nil {
		return nil, err
	}

	ret := make([]Suggestion, 0, len(vals)/div)
	for i := 0; i < len(vals); i += div {

		score, err := strconv.ParseFloat(vals[i+1], 64)
		if err != nil {
			continue
		}

		if payload {
			ret = append(ret, Suggestion{Term: vals[i], Score: score, Payload: vals[i+2]})
		} else {
			ret = append(ret, Suggestion{Term: vals[i], Score: score})
		}
	}

	return ret, nil

}
