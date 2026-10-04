package response

import (
	"encoding/json"
	"testing"
)

func TestResponseContract(t *testing.T) {
	for _, test := range []struct {
		name string
		bean Response
		want string
	}{
		{"success", Success([]int{1, 2}), `{"code":200,"data":[1,2],"msg":"success"}`},
		{"custom message", Success(nil, "saved"), `{"code":200,"data":null,"msg":"saved"}`},
		{"error", Error("failed"), `{"code":500,"data":null,"msg":"failed"}`},
		{"custom error", ErrorCode(404, "missing"), `{"code":404,"data":null,"msg":"missing"}`},
		{"unauthorized", Unauthorized("login required"), `{"code":401,"data":null,"msg":"login required"}`},
		{"expired", SessionExpired("TOKEN_EXPIRED"), `{"code":40101,"data":null,"msg":"登录已超时，请重新登录","reason":"TOKEN_EXPIRED"}`},
		{"empty reason", New(401, nil, "expired", ""), `{"code":401,"data":null,"msg":"expired","reason":""}`},
		{"forbidden", Forbidden("denied"), `{"code":403,"data":null,"msg":"denied"}`},
		{"rate limited", TooManyRequests("slow down"), `{"code":429,"data":null,"msg":"slow down"}`},
	} {
		t.Run(test.name, func(t *testing.T) {
			got, err := json.Marshal(test.bean)
			if err != nil || string(got) != test.want {
				t.Fatalf("got %s, want %s, err=%v", got, test.want, err)
			}
		})
	}
}
