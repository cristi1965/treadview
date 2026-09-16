export interface LatestRequestGate {
  begin: () => number;
  isCurrent: (request: number) => boolean;
  invalidate: () => void;
}

export const createLatestRequestGate = (): LatestRequestGate => {
  let current = 0;
  return {
    begin: () => ++current,
    isCurrent: (request) => request === current,
    invalidate: () => { current += 1; },
  };
};
