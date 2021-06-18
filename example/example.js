const WebSocket = require("ws");
const baseUrl = "https://myrok-some-url.a.run.app";
const ws = new WebSocket(`${baseUrl}/proxy`);

ws.addEventListener("message", (message) => {
	const json = JSON.parse(Buffer.from(message.data).toString("utf8"));
	if (json.endpoint) {
		console.log(`Reach me at ${baseUrl}/${json.endpoint}`);
		return;
	}

	const req = json;
	console.log("request:", req);
	const body = "Hello World";
	const resp = Buffer.from(JSON.stringify({
		uuid: req.uuid,
		status: 200,
		headers: { "Content-Length": body.length },
		body_base64: Buffer.from(body).toString("base64"),
	}));
	console.log("response:", resp);
	ws.send(resp);
});

ws.addEventListener("close", () => {
	console.log(`Proxy closed`);
});

ws.addEventListener("error", (err) => {
	console.log(`Proxy error: ${err}`);
});
