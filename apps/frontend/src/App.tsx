import { BrowserRouter, Route, Routes } from "react-router"
import MainLayout from "@/components/layout/MainLayout"
import FileExplorer from "@/features/browse/components/FileExplorer"

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
      </Routes>
    </BrowserRouter>
  )
}