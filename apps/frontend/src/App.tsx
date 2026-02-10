import { BrowserRouter, Route, Routes } from "react-router"
import MainLayout from "@/components/layout/MainLayout"
import FileExplorer from "@/features/browse/components/FileExplorer"
import Settings from "@/pages/Settings"

export default function App() {
  return (
    <BrowserRouter>
      <Routes>
        <Route element={<MainLayout />}>
          <Route
            path="/"
            element={<FileExplorer />}
          />
        </Route>
        <Route path="/settings" element={<Settings />} />
      </Routes>
    </BrowserRouter>
  )
}