const http = require('node:http');
const net = require('node:net');
for (const [port, host, targetPort] of [[3003, 'web', 3003], [8080, 'api', 8080]]) {
  const server = http.createServer((req, res) => {
    const upstream = http.request({ hostname: host, port: targetPort, method: req.method, path: req.url, headers: req.headers }, incoming => {
      res.writeHead(incoming.statusCode, incoming.headers);
      incoming.pipe(res);
    });
    upstream.on('error', () => { if (!res.headersSent) res.writeHead(502); res.end('Local service starting'); });
    res.on('close', () => upstream.destroy());
    req.pipe(upstream);
  });
  server.on('upgrade', (req, socket, head) => {
    const upstream = net.connect(targetPort, host, () => {
      let headers = `${req.method} ${req.url} HTTP/${req.httpVersion}\r\n`;
      for (let i = 0; i < req.rawHeaders.length; i += 2) headers += `${req.rawHeaders[i]}: ${req.rawHeaders[i + 1]}\r\n`;
      upstream.write(headers + '\r\n');
      if (head.length) upstream.write(head);
      socket.pipe(upstream).pipe(socket);
    });
    upstream.on('error', () => socket.destroy());
    socket.on('error', () => upstream.destroy());
    socket.on('close', () => upstream.destroy());
  });
  server.listen(port, '0.0.0.0', () => console.log(`localhost:${port} -> ${host}:${targetPort}`));
}
