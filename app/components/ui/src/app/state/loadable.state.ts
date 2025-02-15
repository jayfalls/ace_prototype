export interface Loadable {
  loading: boolean;
  error: any;
}

export function createDefaultLoadable(): Loadable {
  return {
    loading: false,
    error: null,
  };
}

export function onLoadableLoad<T extends Loadable>(loadable: T): T {
  return {
    ...(loadable as any),
    loading: true,
    error: null,
  } as T;
}

export function onLoadableSuccess<T extends Loadable>(loadable: T): T {
  return {
    ...(loadable as any),
    loading: false,
    error: null,
  } as T;
}

export function onLoadableError<T extends Loadable>(loadable: T, error: any): T {
  return {
    ...(loadable as any),
    loading: false,
    error: error,
  } as T;
}
