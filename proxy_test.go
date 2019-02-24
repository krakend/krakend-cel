package cel

import (
	"bytes"
	"context"
	"fmt"
	"strconv"
	"testing"
	"time"

	"github.com/devopsfaith/krakend-cel/internal"
	"github.com/devopsfaith/krakend/config"
	"github.com/devopsfaith/krakend/logging"
	"github.com/devopsfaith/krakend/proxy"
)

func TestProxyFactory_reqParams_int(t *testing.T) {
	timeNow = func() time.Time {
		loc, _ := time.LoadLocation("UTC")
		return time.Date(2018, 12, 10, 0, 0, 0, 0, loc)
	}
	defer func() { timeNow = time.Now }()

	buff := bytes.NewBuffer(make([]byte, 1024))
	logger, err := logging.NewLogger("ERROR", buff, "pref")
	if err != nil {
		t.Error("building the logger:", err.Error())
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
		t.Error(err)
		return
	}

	for i := 0; i < 100; i++ {
		resp, err := prxy(context.Background(), &proxy.Request{
			Method:  "GET",
			Path:    "/some-path",
			Params:  map[string]string{"Id": strconv.Itoa(i)},
			Headers: map[string][]string{},
		})

		if i%2 == 0 {
			if err != nil {
				t.Error(err)
				return
			}

			if resp != expectedResponse {
				t.Errorf("unexpected response %+v", resp)
			}
		} else {
			if err == nil {
				t.Error(err)
				return
			}

			if resp != nil {
				t.Errorf("unexpected response %+v", resp)
			}
		}
	}
}

func TestProxyFactory_respParams_int(t *testing.T) {
	timeNow = func() time.Time {
		loc, _ := time.LoadLocation("UTC")
		return time.Date(2018, 12, 10, 0, 0, 0, 0, loc)
	}
	defer func() { timeNow = time.Now }()

	buff := bytes.NewBuffer(make([]byte, 1024))
	logger, err := logging.NewLogger("ERROR", buff, "pref")
	if err != nil {
		t.Error("building the logger:", err.Error())
		return
	}

	pf := proxy.FactoryFunc(func(_ *config.EndpointConfig) (proxy.Proxy, error) {
		return func(ctx context.Context, r *proxy.Request) (*proxy.Response, error) {
			return &proxy.Response{Data: map[string]interface{}{"Id": r.Params["Id"]}, IsComplete: true}, nil
		}, nil
	})

	prxy, err := ProxyFactory(logger, pf).New(&config.EndpointConfig{
		Endpoint: "/",
		ExtraConfig: config.ExtraConfig{
			internal.Namespace: []internal.InterpretableDefinition{
				{CheckExpression: "int(resp_data.Id) % 2 == 0"},
			},
		},
	})
	if err != nil {
		t.Error(err)
		return
	}

	for i := 0; i < 100; i++ {
		resp, err := prxy(context.Background(), &proxy.Request{
			Method:  "GET",
			Path:    "/some-path",
			Params:  map[string]string{"Id": strconv.Itoa(i)},
			Headers: map[string][]string{},
		})

		if i%2 == 0 {
			if err != nil {
				t.Error(err)
				return
			}

			if resp.Data["Id"].(string) != strconv.Itoa(i) {
				t.Errorf("unexpected response %+v", resp)
			}
		} else {
			if err == nil {
				t.Error(err)
				return
			}

			if resp != nil {
				t.Errorf("unexpected response %+v", resp)
			}
		}
	}
}

func TestProxyFactory_reqParams_string(t *testing.T) {
	expectedResponse := &proxy.Response{Data: map[string]interface{}{"ok": true}, IsComplete: true}

	for _, expr := range []string{
		"req_params.Nick in ['kpacha', 'alombarte']",
		"req_params.Nick.matches('kpacha|alombarte')",
		"req_params.Nick.matches('^(kpacha|alombarte)$')",
	} {
		buff := bytes.NewBuffer(make([]byte, 1024))
		logger, err := logging.NewLogger("INFO", buff, "pref")
		if err != nil {
			t.Error("building the logger:", err.Error())
			return
		}

		cfg := &config.EndpointConfig{
			Endpoint: "/",
			ExtraConfig: config.ExtraConfig{
				internal.Namespace: []internal.InterpretableDefinition{{CheckExpression: expr}},
			},
		}

		prxy, err := ProxyFactory(logger, dummyProxyFactory(expectedResponse)).New(cfg)
		if err != nil {
			t.Error(err)
			return
		}

		for i := 0; i < 100; i++ {

			for _, tc := range []struct {
				success bool
				nick    string
			}{
				{success: true, nick: "kpacha"},
				{success: false, nick: "bar"},
				{success: true, nick: "alombarte"},
				{success: false, nick: "foo"},
			} {
				resp, err := prxy(context.Background(), &proxy.Request{
					Method:  "GET",
					Path:    "/some-path",
					Params:  map[string]string{"Nick": tc.nick},
					Headers: map[string][]string{},
				})

				if tc.success {
					if err != nil {
						t.Errorf("#%d (%s - %s) unexpected error: %s", i, expr, tc.nick, err.Error())
						fmt.Println(buff.String())
						return
					}

					if resp != expectedResponse {
						t.Errorf("#%d (%s - %s) wrong response %+v", i, expr, tc.nick, resp)
						fmt.Println(buff.String())
						return
					}
					continue
				}

				if err == nil {
					t.Errorf("#%d (%s - %s) expecting error", i, expr, tc.nick)
					fmt.Println(buff.String())
					return
				}

				if resp != nil {
					t.Errorf("#%d (%s - %s) unexpected response %+v", i, expr, tc.nick, resp)
					fmt.Println(buff.String())
					return
				}
			}
		}
	}
}

func dummyProxyFactory(r *proxy.Response) proxy.Factory {
	return proxy.FactoryFunc(func(_ *config.EndpointConfig) (proxy.Proxy, error) {
		return func(ctx context.Context, _ *proxy.Request) (*proxy.Response, error) {
			return r, nil
		}, nil
	})
}
