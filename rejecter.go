package cel

import (
	"fmt"
	"time"

	"github.com/google/cel-go/cel"
	"github.com/krakend/krakend-cel/v2/internal"
	"github.com/luraproject/lura/v2/config"
	"github.com/luraproject/lura/v2/logging"
)

func NewRejecter(l logging.Logger, cfg *config.EndpointConfig) *Rejecter {
	logPrefix := "[ENDPOINT: " + cfg.Endpoint + "][CEL]"
	def, ok := internal.ConfigGetter(cfg.ExtraConfig)
	if !ok {
		return nil
	}

	p := internal.NewCheckExpressionParser(l)
	evaluators, err := p.ParseJWT(def)
	if err != nil {
		l.Debug(logPrefix, "Error building the JWT rejecter:", err.Error())
		return nil
	}

	return &Rejecter{
		name:       logPrefix,
		evaluators: evaluators,
		logger:     l,
	}
}

type Rejecter struct {
	name       string
	evaluators []cel.Program
	logger     logging.Logger
}

func (r *Rejecter) Reject(data map[string]interface{}) bool {
	now := timeNow().Format(time.RFC3339)
	reqActivation := map[string]interface{}{
		internal.JwtKey: data,
		internal.NowKey: now,
	}
	for i, eval := range r.evaluators {
		res, _, err := eval.Eval(reqActivation)
		if err != nil {
			r.logger.Info(fmt.Sprintf("%s Rejecter #%d failed: %v", r.name, i, res))
			return true
		}

		resultMsg := fmt.Sprintf("%s Rejecter #%d result: %v", r.name, i, res)
		if v, ok := res.Value().(bool); !ok || !v {
			r.logger.Info(resultMsg)
			return true
		}
		r.logger.Debug(resultMsg)
	}
	return false
}
