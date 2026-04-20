import { createBrowserRouter, Navigate, RouterProvider } from "react-router-dom"
import { AppShell } from "@/components/layout/app-shell"
import LoginPage from "@/pages/login"
import NotFoundPage from "@/pages/not-found"
import ProjectListPage from "@/pages/project-list"
import ProjectPage from "@/pages/project-page"
import ProjectEnvPage from "@/pages/project-env-page"
import GlobalValuesListPage from "@/pages/global-values-list"
import GlobalValuesDetailPage from "@/pages/global-values-detail"
import CreatePRPage from "@/pages/create-pr"
import PullRequestsPage from "@/pages/pull-requests-page"
import PRDetailPage from "@/pages/pr-detail-page"
import WorkspacePage from "@/pages/workspace-page"
import WorkspaceProjectPage from "@/pages/workspace-project-page"
import WorkspaceEnvPage from "@/pages/workspace-env-page"
import DeployPage from "@/pages/deploy-page"

const router = createBrowserRouter([
  {
    path: "/",
    element: <AppShell />,
    children: [
      { index: true, element: <Navigate to="/projects" replace /> },
      { path: "projects", element: <ProjectListPage /> },
      { path: "projects/:name", element: <ProjectPage /> },
      { path: "projects/:name/env/:env", element: <ProjectEnvPage /> },
      { path: "global-values", element: <GlobalValuesListPage /> },
      { path: "global-values/:name", element: <GlobalValuesDetailPage /> },
      { path: "global-values/:name/create-pr", element: <CreatePRPage /> },
      { path: "pull-requests", element: <PullRequestsPage /> },
      { path: "pull-requests/:id", element: <PRDetailPage /> },
      { path: "deploy", element: <DeployPage /> },
      { path: "workspace", element: <WorkspacePage /> },
      { path: "workspace/:name", element: <WorkspaceProjectPage /> },
      { path: "workspace/:name/env/:env", element: <WorkspaceEnvPage /> },
    ],
  },
  { path: "/login", element: <LoginPage /> },
  { path: "*", element: <NotFoundPage /> },
])

export default function App() {
  return <RouterProvider router={router} />
}
