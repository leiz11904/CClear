import { Shell } from "./components/Shell";
import { StoreProvider } from "./state/store";

export function App() {
  return (
    <StoreProvider>
      <Shell />
    </StoreProvider>
  );
}
