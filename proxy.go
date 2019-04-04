package cel

import (
	"context"
	"fmt"
	"time"

	"github.com/devopsfaith/krakend-cel/internal"
	"github.com/devopsfaith/krakend/config"
	"github.com/devopsfaith/krakend/logging"
	"github.com/devopsfaith/krakend/proxy"
	"github.com/google/cel-go/cel"
)

func ProxyFactory(l logging.Logger, pf proxy.Factory) proxy.Factory {
	return proxy.FactoryFunc(func(cfg *config.EndpointConfig) (proxy.Proxy, error) {
		next, err := pf.New(cfg)
		if err != nil {
			return next, err
		}

		def, ok := internal.ConfigGetter(cfg.ExtraConfig)
		if !ok {
			l.Debug("CEL: no extra config detected for pipe", cfg.Endpoint)
			return next, nil
		}
		l.Debug("CEL: loading the extra config detected for pipe", cfg.Endpoint)

		p, err := newProxy(l, "proxy "+cfg.Endpoint, def, next)
		if err != nil {
			l.Warning("CEL: error parsing the definitions for pipe", cfg.Endpoint, ":", err.Error())
			l.Warning("CEL: falling back to the next pipe proxy")
			return next, nil
		}
		return p, err
	})
}

func BackendFactory(l logging.Logger, bf proxy.BackendFactory) proxy.BackendFactory {
	return func(cfg *config.Backend) proxy.Proxy {
		next := bf(cfg)

		def, ok := internal.ConfigGetter(cfg.ExtraConfig)
		if !ok {
			l.Debug("CEL: no extra config detected for backend", cfg.URLPattern)
			return next
		}
		l.Debug("CEL: loading the extra config detected for backend", cfg.URLPattern)

		p, err := newProxy(l, "backend "+cfg.URLPattern, def, next)
		if err != nil {
			l.Warning("CEL: error parsing the definitions for backend", cfg.URLPattern, ":", err.Error())
			l.Warning("CEL: falling back to the next backend proxy")
			return next
		}
		return p
	}
}

func newProxy(l logging.Logger, name string, defs []internal.InterpretableDefinition, next proxy.Proxy) (proxy.Proxy, error) {
	p := internal.NewCheckExpressionParser(l)
	preEvaluators, err := p.ParsePre(defs)
	if err != nil {
		return proxy.NoopProxy, err
	}
	postEvaluators, err := p.ParsePost(defs)
	if err != nil {
		return proxy.NoopProxy, err
	}

	l.Debug("CEL:", name, "preEvaluators", preEvaluators)
	l.Debug("CEL:", name, "postEvaluators", postEvaluators)

	return func(ctx context.Context, r *proxy.Request) (*proxy.Response, error) {
		now := timeNow().Format(time.RFC3339)

		if err := evalChecks(l, name+"-pre", newReqActivation(r, now), preEvaluators); err != nil {
			return nil, err
		}

		resp, err := next(ctx, r)
		if err != nil {
			l.Debug(fmt.Sprintf("CEL: %s delegated execution failed: %s", name, err.Error()))
			return resp, err
		}

		if err := evalChecks(l, name+"-post", newRespActivation(resp, now), postEvaluators); err != nil {
			return nil, err
		}

		return resp, nil
	}, nil
}

func evalChecks(l logging.Logger, name string, args map[string]interface{}, ps []cel.Program) error {
	for i, eval := range ps {
		res, _, err := eval.Eval(args)
		resultMsg := fmt.Sprintf("CEL: %s evaluator #%d result: %v - err: %v", name, i, res, err)

		if v, ok := res.Value().(bool); !ok || !v {
			l.Info(resultMsg)
			return fmt.Errorf("CEL: request aborted by %+v", eval)
		}
		l.Debug(resultMsg)
	}
	return nil
}

func newReqActivation(r *proxy.Request, now string) map[string]interface{} {
	return map[string]interface{}{
		internal.PreKey + "_method":  r.Method,
		internal.PreKey + "_path":    r.Path,
		internal.PreKey + "_params":  r.Params,
		internal.PreKey + "_headers": r.Headers,
		internal.NowKey:              now,
	}
}

func newRespActivation(r *proxy.Response, now string) map[string]interface{} {
	return map[string]interface{}{
		internal.PostKey + "_completed":        r.IsComplete,
		internal.PostKey + "_metadata_status":  r.Metadata.StatusCode,
		internal.PostKey + "_metadata_headers": r.Metadata.Headers,
		internal.PostKey + "_data":             r.Data,
		internal.NowKey:                        now,
	}
}

var timeNow = time.Now
