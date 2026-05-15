import express from 'express';
import pino from 'pino';
import client from 'prom-client';

const log = pino({ level: process.env.LOG_LEVEL || 'info' });
const app = express();
app.use(express.json());

const reg = new client.Registry();
client.collectDefaultMetrics({ register: reg });
const reqHist = new client.Histogram({
  name: 'http_request_duration_seconds',
  help: 'Request latency',
  labelNames: ['route', 'status'],
  registers: [reg],
});
const reqCount = new client.Counter({
  name: 'http_requests_total',
  help: 'Request count',
  labelNames: ['route', 'status'],
  registers: [reg],
});

app.use((req, res, next) => {
  const start = Date.now();
  res.on('finish', () => {
    const dur = (Date.now() - start) / 1000;
    reqHist.labels(req.path, res.statusCode).observe(dur);
    reqCount.labels(req.path, res.statusCode).inc();
  });
  next();
});

const products = [
  { id: 1, name: 'Widget',  price: 9.99 },
  { id: 2, name: 'Gizmo',   price: 14.99 },
  { id: 3, name: 'Gadget',  price: 24.99 },
];

app.get('/healthz', (_, res) => res.send('ok'));
app.get('/readyz',  (_, res) => res.send('ready'));
app.get('/products', (_, res) => res.json(products));
app.get('/products/:id', (req, res) => {
  const p = products.find((x) => x.id === Number(req.params.id));
  return p ? res.json(p) : res.status(404).json({ error: 'not found' });
});
app.get('/metrics', async (_, res) => {
  res.set('Content-Type', reg.contentType);
  res.send(await reg.metrics());
});

const port = process.env.PORT || 3000;
const server = app.listen(port, () => log.info({ port }, 'product-service listening'));

['SIGTERM', 'SIGINT'].forEach((sig) =>
  process.on(sig, () => {
    log.info({ sig }, 'shutting down');
    server.close(() => process.exit(0));
  }),
);
