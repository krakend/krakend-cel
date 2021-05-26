package internal

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"strings"

	"github.com/google/cel-go/cel"
	"github.com/google/cel-go/checker/decls"
	"github.com/luraproject/lura/config"
	"github.com/luraproject/lura/logging"
)

type InterpretableDefinition struct {
	CheckExpression string `json:"check_expr"`
	ModExpression   string `json:"mod_expr"`
}

func ConfigGetter(e config.ExtraConfig) ([]InterpretableDefinition, bool) {
	def := []InterpretableDefinition{}
	v, ok := e[Namespace]
	if !ok {
		return def, ok
	}
	buf := new(bytes.Buffer)
	if err := json.NewEncoder(buf).Encode(&v); err != nil {
		return def, false
	}

	if err := json.NewDecoder(buf).Decode(&def); err != nil {
		return def, false
	}
	return def, true
}

const Namespace = "github.com/devopsfaith/krakend-cel"

var (
	ErrParsing  = errors.New("cel: error parsing the expression")
	ErrChecking = errors.New("cel: error checking the expression and its param definition")
	ErrNoExpr   = errors.New("cel: no expression")
)

func NewCheckExpressionParser(l logging.Logger) Parser {
	return Parser{
		extractor: extractCheckExpr,
		w:         &logger{l},
	}
}

func NewModExpressionParser(l logging.Logger) Parser {
	return Parser{
		extractor: extractModExpr,
		w:         &logger{l},
	}
}

type Parser struct {
	extractor func(InterpretableDefinition) string
	w         io.Writer
}

func (p Parser) Parse(definition InterpretableDefinition) (cel.Program, error) {
	expr := p.extractor(definition)
	if expr == "" {
		return nil, ErrNoExpr
	}
	fmt.Println(expr)
	env, err := cel.NewEnv(defaultDeclarations())
	if err != nil {
		fmt.Println(err.Error())
		return nil, err
	}

	ast, iss := env.Parse(p.extractor(definition))
	if iss != nil && iss.Err() != nil {
		fmt.Println(iss.Err())
		return nil, ErrParsing
	}
	c, iss := env.Check(ast)
	if iss != nil && iss.Err() != nil {
		fmt.Fprintln(p.w, iss.Err())
		return nil, ErrChecking
	}

	return env.Program(c)
}

func (p Parser) ParsePre(definitions []InterpretableDefinition) ([]cel.Program, error) {
	return p.parseByKey(definitions, PreKey)
}

func (p Parser) ParsePost(definitions []InterpretableDefinition) ([]cel.Program, error) {
	return p.parseByKey(definitions, PostKey)
}

func (p Parser) ParseJWT(definitions []InterpretableDefinition) ([]cel.Program, error) {
	return p.parseByKey(definitions, JwtKey)
}

func (p Parser) parseByKey(definitions []InterpretableDefinition, key string) ([]cel.Program, error) {
	res := []cel.Program{}
	for _, def := range definitions {
		if !strings.Contains(p.extractor(def), key) {
			continue
		}
		v, err := p.Parse(def)
		if err == ErrNoExpr {
			continue
		}

		if err != nil {
			return res, err
		}
		res = append(res, v)
	}
	return res, nil
}

func defaultDeclarations() cel.EnvOption {
	return cel.Declarations(
		decls.NewIdent(NowKey, decls.String, nil),

		decls.NewIdent(PreKey+"_method", decls.String, nil),
		decls.NewIdent(PreKey+"_path", decls.String, nil),
		decls.NewIdent(PreKey+"_params", decls.NewMapType(decls.String, decls.String), nil),
		decls.NewIdent(PreKey+"_headers", decls.NewMapType(decls.String, decls.NewListType(decls.String)), nil),
		decls.NewIdent(PreKey+"_querystring", decls.NewMapType(decls.String, decls.NewListType(decls.String)), nil),

		decls.NewIdent(PostKey+"_completed", decls.Bool, nil),
		decls.NewIdent(PostKey+"_metadata_status", decls.Int, nil),
		decls.NewIdent(PostKey+"_metadata_headers", decls.NewMapType(decls.String, decls.NewListType(decls.String)), nil),
		decls.NewIdent(PostKey+"_data", decls.NewMapType(decls.String, decls.Dyn), nil),

		decls.NewIdent(JwtKey, decls.NewMapType(decls.String, decls.Dyn), nil),
	)
}

func extractCheckExpr(i InterpretableDefinition) string { return i.CheckExpression }
func extractModExpr(i InterpretableDefinition) string   { return i.ModExpression }

const (
	PreKey  = "req"
	PostKey = "resp"
	JwtKey  = "JWT"
	NowKey  = "now"
)

type logger struct {
	l logging.Logger
}

func (l *logger) Write(p []byte) (n int, err error) {
	l.l.Debug("CEL:", string(p))
	return len(p), nil
}

var _ io.Writer = &logger{}
