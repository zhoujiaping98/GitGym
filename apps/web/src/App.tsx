import { LoginScreen } from "./components/LoginScreen";
import { TopBar } from "./components/TopBar";
import { Workbench } from "./components/Workbench";

export default function App() {
  return (
    <div className="app-shell">
      <TopBar />
      <LoginScreen preview={<Workbench preview />} />
    </div>
  );
}
