import contract from './console-contract.json';

export type ConsoleHTTPMethod = 'DELETE' | 'GET' | 'POST' | 'PUT';

export interface ConsoleOperation {
  method: ConsoleHTTPMethod;
  operationId: string;
  path: string;
}

interface ConsoleAPIContract {
  basePath: string;
  operations: ConsoleOperation[];
}

type PathParameters = Record<string, number | string | undefined>;

const pathParameterPattern = /\{([^}]+)\}/g;

export const consoleAPIContract = contract as ConsoleAPIContract;

const operationsByID = new Map(
  consoleAPIContract.operations.map((operation) => [
    operation.operationId,
    operation,
  ]),
);

export function consoleOperation(operationId: string): ConsoleOperation {
  const operation = operationsByID.get(operationId);
  if (!operation) {
    throw new Error(`Unknown Console operationId: ${operationId}`);
  }
  return operation;
}

export function consoleEndpoint(
  operationId: string,
  parameters: PathParameters = {},
): string {
  const operation = consoleOperation(operationId);
  const path = operation.path.replaceAll(
    pathParameterPattern,
    (_, name: string) => {
      const value = parameters[name];
      if (value === undefined || value === '') {
        throw new Error(
          `Missing path parameter "${name}" for Console operationId: ${operationId}`,
        );
      }
      return encodeURIComponent(String(value));
    },
  );

  return `${consoleAPIContract.basePath}${path}`;
}
