// preact-router injects `path`, `matches`, `url` etc. as props.
// This type helper allows components to receive them without TS errors.
export interface RoutableProps {
  path?: string;
  default?: boolean;
  url?: string;
  matches?: Record<string, string>;
}
