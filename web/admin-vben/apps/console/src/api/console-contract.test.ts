import { describe, expect, it } from 'vitest';

import {
  consoleAPIContract,
  consoleEndpoint,
  consoleOperation,
} from './console-contract';

const apiModules = import.meta.glob('./**/*.ts', {
  eager: true,
  import: 'default',
  query: '?raw',
}) as Record<string, string>;

describe('console API endpoint registry', () => {
  it('has unique OpenAPI operation IDs and canonical relative paths', () => {
    const identifiers = consoleAPIContract.operations.map(
      ({ operationId }) => operationId,
    );
    const endpointKeys = consoleAPIContract.operations.map(
      ({ method, path }) => `${method} ${path}`,
    );

    expect(consoleAPIContract.basePath).toBe('/console/v1');
    expect(new Set(identifiers)).toHaveLength(identifiers.length);
    expect(new Set(endpointKeys)).toHaveLength(endpointKeys.length);
    for (const operation of consoleAPIContract.operations) {
      expect(operation.operationId).toMatch(/^console[A-Z]/);
      expect(operation.method).toMatch(/^(DELETE|GET|POST|PUT)$/);
      expect(operation.path).toMatch(/^\//);
    }
  });

  it('builds encoded paths from an OpenAPI operation ID', () => {
    expect(consoleOperation('consoleGetOperationLog')).toMatchObject({
      method: 'GET',
      path: '/logs/operations/{id}',
    });
    expect(consoleEndpoint('consoleGetOperationLog', { id: 'log/a b' })).toBe(
      '/console/v1/logs/operations/log%2Fa%20b',
    );
    expect(() => consoleEndpoint('consoleGetOperationLog')).toThrow(
      'Missing path parameter "id"',
    );
    expect(() => consoleEndpoint('consoleUnknownOperation')).toThrow(
      'Unknown Console operationId',
    );
  });

  it('only calls operations the registry knows', () => {
    for (const [filename, source] of Object.entries(apiModules)) {
      if (filename.endsWith('.test.ts')) {
        continue;
      }
      const calls = source.matchAll(/consoleEndpoint\(\s*'([^']+)'/g);
      for (const [, operationId = ''] of calls) {
        expect(() => consoleOperation(operationId), filename).not.toThrow();
      }
    }
  });

  it('keeps Console API implementations free of inline endpoint guesses', () => {
    for (const [filename, source] of Object.entries(apiModules)) {
      if (filename.endsWith('.test.ts')) {
        continue;
      }
      expect(source, filename).not.toMatch(/['"`]\/console\/v1/);
      if (source.includes('requestClient.')) {
        expect(source, filename).toContain('consoleEndpoint(');
      }
    }
  });
});
