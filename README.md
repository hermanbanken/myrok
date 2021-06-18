# Cloud Run ngrok alternative
Serverless hosted ngrok alternative.

1. Connect to `/proxy` using WebSockets.
2. You receive `{ "endpoint": "<some-uuid> " }`
3. Start sending requests to `/<some-uuid>/...rest`
4. On WebSockets you receive requests & you reply with responses.

Request format:

```javascript
{
	"uuid": "some-uuid",
	"method": "POST",
	"path": "/",
	"headers": {
		"Host": ["example.org"],
		"Content-Length": ["11"]
	},
	"body_base64": "SGVsbG8gV29ybGQK" # "Hello World"
}
```

Response format:

```json
{
	"uuid": "some-uuid",
	"status": "200",
	"headers": {
		"Host": ["example.org"],
		"Content-Length": ["1"]
	},
	"body_base64": "SGVsbG8gQmFjawo=" # "Hello Back"
}
```
