import { BrowserRouter, Route, Routes } from "react-router"
import MainLayout from "@/components/layout/MainLayout"
import FileExplorer from "@/features/browse/components/FileExplorer"
import Settings from "@/pages/Settings"
import Login from "@/pages/Login"
import RequireAuth from "@/features/auth/RequireAuth"

export default function App() {
  return (
    <BrowserRouter>
      <Routes>
        <Route path="/login" element={<Login />} />
        <Route element={<RequireAuth><MainLayout /></RequireAuth>}>
          <Route
            path="/"
            element={<FileExplorer />}
          />
          <Route
            path="/search"
            element={<FileExplorer />}
          />
        </Route>
        <Route
          path="/settings"
          element={<RequireAuth><Settings /></RequireAuth>}
        />
      </Routes>
    </BrowserRouter>
  )
}
