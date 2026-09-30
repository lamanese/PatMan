import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import { render, screen, waitFor } from "@testing-library/react";
import { MemoryRouter } from "react-router-dom";
import { describe, expect, it, vi } from "vitest";
import { ToastProvider } from "../../contexts/ToastContext";
import Patching from "../../pages/Patching";
import { patchingAPI } from "../../utils/patchingApi";

vi.mock("../../utils/patchingApi", () => ({
	patchingAPI: {
		getDashboard: vi.fn(() =>
			Promise.resolve({ summary: {}, recent_runs: [], active_runs: [] }),
		),
		getRuns: vi.fn(),
		getActiveRuns: vi.fn(() => Promise.resolve({ runs: [] })),
		getPolicies: vi.fn(() => Promise.resolve([])),
		deleteRun: vi.fn(),
		approveRun: vi.fn(),
		retryValidation: vi.fn(),
		bulkSolveRuns: vi.fn(),
	},
	buildRunStreamURL: () => "ws://localhost/none",
}));
vi.mock("../../utils/api", () => ({
	adminHostsAPI: { list: vi.fn(() => Promise.resolve({ data: { data: [] } })) },
	hostGroupsAPI: { list: vi.fn(() => Promise.resolve({ data: [] })) },
	formatDate: (d) => String(d),
	default: { get: vi.fn(() => Promise.resolve({ data: {} })) },
}));
vi.mock("../../contexts/AuthContext", () => ({
	useAuth: () => ({
		hasModule: () => true,
		hasPermission: () => true,
		canManagePatching: () => true,
		canViewHosts: () => true,
		user: { role: "admin" },
	}),
}));
vi.mock("../../contexts/SettingsContext", () => ({
	useSettings: () => ({ settings: {} }),
}));
vi.mock("../../contexts/ThemeContext", () => ({
	useTheme: () => ({ isDark: false }),
}));
vi.mock("react-chartjs-2", () => ({
	Bar: () => null,
	Doughnut: () => null,
	Line: () => null,
	Pie: () => null,
}));

const failedRun = {
	id: "11111111-1111-4111-8111-111111111111",
	host_id: "h1",
	patch_type: "patch_all",
	status: "failed",
	created_at: "2026-09-28T10:00:00Z",
	completed_at: "2026-09-28T10:05:00Z",
	hosts: { id: "h1", friendly_name: "web01", hostname: "web01" },
};

const renderPage = (runs) => {
	patchingAPI.getRuns.mockResolvedValue({
		runs,
		pagination: { total: runs.length, pages: 1, page: 1, limit: 25 },
	});
	render(
		<QueryClientProvider
			client={
				new QueryClient({ defaultOptions: { queries: { retry: false } } })
			}
		>
			<ToastProvider>
				<MemoryRouter initialEntries={["/patching?tab=runs"]}>
					<Patching />
				</MemoryRouter>
			</ToastProvider>
		</QueryClientProvider>,
	);
};

describe("Patching page", () => {
	it("renders the runs tab without any run", async () => {
		renderPage([]);
		await waitFor(() =>
			expect(
				screen.getByRole("option", { name: "Solved" }),
			).toBeInTheDocument(),
		);
	});

	it("offers Mark solved and the solve checkbox on a failed run", async () => {
		renderPage([failedRun]);
		await waitFor(() =>
			expect(
				screen.getAllByRole("button", { name: /Mark solved/i }).length,
			).toBeGreaterThan(0),
		);
		expect(
			screen.getAllByLabelText(/Select to mark as solved/i).length,
		).toBeGreaterThan(0);
	});
});
