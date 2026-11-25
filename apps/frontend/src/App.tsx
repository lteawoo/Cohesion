import { BrowserRouter, Route, Routes } from "react-router"
import MainLayout from "@/components/layout/MainLayout"
import FileExplorer from "@/features/browse/components/FileExplorer"
import TrashExplorer from "@/features/browse/components/TrashExplorer"
import Settings from "@/pages/Settings"
import SharedDownload from "@/features/downloadLinks/SharedDownload"
import FileDrop from "@/features/fileRequests/FileDrop"
import Login from "@/pages/Login"
import RequireAuth from "@/features/auth/RequireAuth"

export default function App() {
  return (
    <BrowserRouter>
      <Routes>
        <Route path="/share" element={<SharedDownload />} />
        <Route path="/request" element={<FileDrop />} />
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
          <Route
            path="/trash"
            element={<TrashExplorer />}
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
