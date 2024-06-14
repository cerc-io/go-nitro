import prom, {Counter} from 'prom-client';

import {Logger} from '../log/logger.js';

const log = new Logger('cerc:nitro:auth:metrics');

const prefix = 'nitro_auth_';
prom.collectDefaultMetrics({ prefix });

export class Metrics {
  private readonly counters: Map<string, Counter> = new Map<string, Counter>;

  createCounter(name: string, labelNames = ['method', 'user'], help?: string) {
    const baseName = `${prefix}${name}`;
    this.counters.set(baseName, new prom.Counter({name: baseName, labelNames, help: help ?? name}));
    this.counters.set(`${baseName}_total`, new prom.Counter({name: `${baseName}_total`, help: help ?? name}));
  }

  incCounter(name: string, labels?: any, value = 1) {
    const baseName = `${prefix}${name}`;
    log.trace(`inc ${name}:${labels}(${value})`);
    if (!this.counters.has(baseName)) {
      if (labels) {
        this.createCounter(name, Object.keys(labels));
      } else {
        this.createCounter(name);
      }
    }

    if (labels) {
      this.counters.get(baseName)?.labels(labels).inc(value);
    }

    this.counters.get(`${prefix}${name}_total`)?.inc(value);
  }

  async render(): Promise<string> {
    return prom.register.metrics();
  }
}
