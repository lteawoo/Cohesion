import { BrowserRouter, Route, Routes } from "react-router"
import MainLayout from "@/components/layout/MainLayout"
import { Button } from "antd"

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
                <Button type="primary" onClick={async () => {
                  const res = await fetch('/api/health')
                    .then(response => response.json())
                  console.log(res)
                }}>api test</Button>
              </div>
            }
          />
        </Route>
      </Routes>
    </BrowserRouter>
  )
}