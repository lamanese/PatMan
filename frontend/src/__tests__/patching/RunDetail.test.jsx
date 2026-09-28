import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import { render, screen, waitFor } from "@testing-library/react";
import { MemoryRouter, Route, Routes } from "react-router-dom";
import { describe, expect, it, vi } from "vitest";
import { ToastProvider } from "../../contexts/ToastContext";
import RunDetail from "../../pages/patching/RunDetail";
import { patchingAPI } from "../../utils/patchingApi";

vi.mock("../../utils/patchingApi", () => ({
	patchingAPI: {
		getRunById: vi.fn(),
		approveRun: vi.fn(),
		retryValidation: vi.fn(),
		stopRun: vi.fn(),
		solveRun: vi.fn(),
		reopenRun: vi.fn(),
		updateSolvedNote: vi.fn(),
	},
	buildRunStreamURL: () => "ws://localhost/none",
}));
vi.mock("../../utils/api", () => ({
	formatDate: (d) => String(d),
	default: { get: vi.fn(() => Promise.resolve({ data: {} })) },
}));

const OUTPUT =
	"E: dpkg was interrupted\n[PatchMon] Suggested fix:\n  sudo dpkg --configure -a\n";

const baseRun = (status) => ({
	id: "11111111-1111-4111-8111-111111111111",
	host_id: "h1",
	patch_type: "patch_all",
	status,
	shell_output: OUTPUT,
	error_message: "exit 100",
	dry_run: false,
	created_at: "2026-09-28T10:00:00Z",
	completed_at: "2026-09-28T10:05:00Z",
	hosts: { id: "h1", friendly_name: "web01", hostname: "web01" },
	fork_solved_at: null,
	fork_solved_by: null,
	fork_solved_note: null,
	fork_solved_by_run_id: null,
});

const renderRun = (run) => {
	patchingAPI.getRunById.mockResolvedValue(run);
	render(
		<QueryClientProvider
			client={
				new QueryClient({ defaultOptions: { queries: { retry: false } } })
			}
		>
			<ToastProvider>
				<MemoryRouter initialEntries={[`/patching/runs/${run.id}`]}>
					<Routes>
						<Route path="/patching/runs/:id" element={<RunDetail />} />
					</Routes>
				</MemoryRouter>
			</ToastProvider>
		</QueryClientProvider>,
	);
};

describe("RunDetail solved runs", () => {
	it("offers Mark as solved on a failed run", async () => {
		renderRun(baseRun("failed"));
		await waitFor(() =>
			expect(
				screen.getByRole("button", { name: /Mark as solved/i }),
			).toBeInTheDocument(),
		);
	});

	it("shows the solved box, keeps the failure output and its copyable fix", async () => {
		renderRun({
			...baseRun("solved"),
			fork_solved_at: "2026-09-28T12:00:00Z",
			fork_solved_by: "user-9",
			fork_solved_by_username: "ops",
			fork_solved_note: "Reconfigured dpkg by hand",
		});
		await waitFor(() =>
			expect(
				screen.getByRole("button", { name: /Reopen/i }),
			).toBeInTheDocument(),
		);
		expect(screen.getByText("Reconfigured dpkg by hand")).toBeInTheDocument();
		expect(screen.getByText(/ops/)).toBeInTheDocument();
		expect(screen.getByText(/E: dpkg was interrupted/)).toBeInTheDocument();
		expect(
			screen.getAllByText(/sudo dpkg --configure -a/).length,
		).toBeGreaterThan(0);
		expect(
			screen.queryByRole("button", { name: /Mark as solved/i }),
		).not.toBeInTheDocument();
	});
});
