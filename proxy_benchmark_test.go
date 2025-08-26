package cel

import (
	"bytes"
	"context"
	"strconv"
	"testing"

	"github.com/krakend/krakend-cel/v2/internal"
	"github.com/luraproject/lura/v2/config"
	"github.com/luraproject/lura/v2/logging"
	"github.com/luraproject/lura/v2/proxy"
)

func BenchmarkProxyFactory_reqParams_int(b *testing.B) {
	buff := bytes.NewBuffer(make([]byte, 1024))
	logger, err := logging.NewLogger("ERROR", buff, "pref")
	if err != nil {
		b.Error("building the logger:", err.Error())
		return
	}

	expectedResponse := &proxy.Response{Data: map[string]interface{}{"ok": true}, IsComplete: true}

	prxy, err := ProxyFactory(logger, dummyProxyFactory(expectedResponse)).New(&config.EndpointConfig{
		Endpoint: "/",
		ExtraConfig: config.ExtraConfig{
			internal.Namespace: []internal.InterpretableDefinition{
				{CheckExpression: "int(req_params.Id) % 2 == 0"},
			},
		},
	})
	if err != nil {
		b.Error(err)
		return
	}

	b.ResetTimer()
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		prxy(context.Background(), &proxy.Request{
			Method:  "GET",
			Path:    "/some-path",
			Params:  map[string]string{"Id": strconv.Itoa(i)},
			Headers: map[string][]string{},
		})
	}
}
