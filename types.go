package interpolate

import (
	"encoding/json"
	"fmt"
	"reflect"
	"regexp"
	"strings"

	"github.com/hashicorp/go-multierror"
)

var (
	// TODO: does this need to match regex in concourse/concourse?
	// TODO: this will allow "((" inside a "((...))"
	interpolationRegex         = regexp.MustCompile(`\(\(.+\)\)`)
	interpolationAnchoredRegex = regexp.MustCompile("\\A" + interpolationRegex.String() + "\\z")
)

var (
	errAssignedNotAVar             = fmt.Errorf("assigned value is not a var reference")
	errInvalidTypeForInterpolation = func(t reflect.Type) error {
		return fmt.Errorf("cannot interpolate %s into a string (only strings, numbers, and bools are supported)", t.Kind().String())
	}
)

type Resolver interface {
	Resolve(varName string) (interface{}, error)
}

type String string

func (s String) Interpolate(resolver Resolver) (string, error) {
	if interpolationAnchoredRegex.MatchString(string(s)) {
		var dst string
		if err := Var(s).InterpolateInto(resolver, &dst); err != nil {
			return "", err
		}
		return dst, nil
	}
	var merr error
	interpolated := interpolationRegex.ReplaceAllStringFunc(string(s), func(name string) string {
		name = stripParens(name)

		var val interface{}
		if err := Var(name).InterpolateInto(resolver, &val); err != nil {
			merr = multierror.Append(merr, err)
			return name
		}

		switch val := val.(type) {
		case string, float64, bool:
			return fmt.Sprint(val)
		default:
			merr = multierror.Append(merr, errInvalidTypeForInterpolation(reflect.TypeOf(val)))
			return name
		}
	})

	return interpolated, merr
}

type Var string

func (v *Var) UnmarshalJSON(data []byte) error {
	var dst string
	if err := json.Unmarshal(data, &dst); err != nil {
		return err
	}
	if !interpolationAnchoredRegex.MatchString(dst) {
		return errAssignedNotAVar
	}
	*v = Var(stripParens(dst))
	return nil
}

func (v Var) MarshalJSON() ([]byte, error) {
	return json.Marshal("((" + string(v) + "))")
}

func (v Var) InterpolateInto(resolver Resolver, dst interface{}) error {
	val, err := resolver.Resolve(string(v))
	if err != nil {
		return err
	}
	payload, err := json.Marshal(val)
	if err != nil {
		return err
	}
	if err := json.Unmarshal(payload, &dst); err != nil {
		return err
	}
	return nil
}

func stripParens(name string) string {
	name = strings.TrimPrefix(name, "((")
	name = strings.TrimSuffix(name, "))")
	return name
}
