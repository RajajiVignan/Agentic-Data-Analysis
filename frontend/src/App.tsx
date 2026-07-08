import { ToastProvider } from "@/components/ToastProvider";
import Workspace from "@/app/page";

export default function App() {
  return (
    <ToastProvider>
      <Workspace />
    </ToastProvider>
  );
}
