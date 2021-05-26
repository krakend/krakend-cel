package cel

import (
	"bytes"
	"fmt"
	"testing"
	"time"

	"github.com/devopsfaith/krakend-cel/internal"
	"github.com/luraproject/lura/config"
	"github.com/luraproject/lura/logging"
)

func TestRejecter_Reject(t *testing.T) {
	timeNow = func() time.Time {
		loc, _ := time.LoadLocation("UTC")
		return time.Date(2018, 12, 10, 0, 0, 0, 0, loc)
	}
	defer func() { timeNow = time.Now }()

	buff := bytes.NewBuffer(make([]byte, 1024))
	logger, err := logging.NewLogger("DEBUG", buff, "pref")
	if err != nil {
		t.Error("building the logger:", err.Error())
		return
	}

	rejecter := NewRejecter(logger, &config.EndpointConfig{
		Endpoint: "/",
		ExtraConfig: config.ExtraConfig{
			internal.Namespace: []internal.InterpretableDefinition{
				{CheckExpression: "has(JWT.user_id) && has(JWT.enabled_days) && (timestamp(now).getDayOfWeek() in JWT.enabled_days)"},
			},
		},
	})

	defer func() {
		fmt.Println(buff.String())
	}()

	if rejecter == nil {
		t.Error("nil rejecter")
		return
	}

	for _, tc := range []struct {
		data     map[string]interface{}
		expected bool
	}{
		{
			data:     map[string]interface{}{},
			expected: true,
		},
		{
			data: map[string]interface{}{
				"user_id": 1,
			},
			expected: true,
		},
		{
			data: map[string]interface{}{
				"user_id":      1,
				"enabled_days": []int{},
			},
			expected: true,
		},
		{
			data: map[string]interface{}{
				"user_id":      1,
				"enabled_days": []int{1, 2, 3, 4, 5},
			},
			expected: false,
		},
	} {
		if res := rejecter.Reject(tc.data); res != tc.expected {
			t.Errorf("%+v => unexpected response %v", tc.data, res)
		}
	}
}
