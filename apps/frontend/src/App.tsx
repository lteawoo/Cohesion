import { BrowserRouter, Route, Routes } from "react-router"
import MainLayout from "@/components/layout/MainLayout"

export default function App() {
  return (
    <BrowserRouter>
      <Routes>
        <Route element={<MainLayout />}>
          <Route
            path="/"
            element={
              <div>
                Hello Cohesion!
              </div>
            }
          />
        </Route>
      </Routes>
    </BrowserRouter>
  )
}