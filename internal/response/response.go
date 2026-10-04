package response

// Response is the common API response envelope.
type Response struct {
	Code   int         `json:"code"`
	Data   interface{} `json:"data"`
	Msg    string      `json:"msg"`
	Reason *string     `json:"reason,omitempty"`
}

func New(code int, data interface{}, msg string, reason ...string) Response {
	result := Response{Code: code, Data: data, Msg: msg}
	if len(reason) > 0 {
		result.Reason = &reason[0]
	}
	return result
}

func Success(data interface{}, message ...string) Response {
	msg := "success"
	if len(message) > 0 {
		msg = message[0]
	}
	return New(200, data, msg)
}

func Error(message string) Response { return ErrorCode(500, message) }

func ErrorCode(code int, message string) Response { return New(code, nil, message) }

func Unauthorized(message string) Response { return ErrorCode(401, message) }

func SessionExpired(reason string) Response {
	return New(40101, nil, "登录已超时，请重新登录", reason)
}

func Forbidden(message string) Response { return ErrorCode(403, message) }

func TooManyRequests(message string) Response { return ErrorCode(429, message) }
