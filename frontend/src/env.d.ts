export {};

declare global {
  interface Window {
    go?: {
      main?: {
        Controller?: Record<string, (...args: any[]) => Promise<any>>;
      };
    };
  }
}
