import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import { render, screen, waitFor } from "@testing-library/react";
import { afterEach, describe, expect, it, vi } from "vitest";
import { ToastProvider } from "../../contexts/ToastContext";
import ReportArchiveDialog from "../../pages/settings/ReportArchiveDialog";
import { notificationsAPI } from "../../utils/api";

vi.mock("../../utils/api", () => ({
	notificationsAPI: {
		listReportArchive: vi.fn(),
		downloadReportArchivePdf: vi.fn(),
	},
	formatDate: (d) => String(d),
	formatDateOnly: (d) => String(d),
}));

const run = (status) => ({
	id: `run-${status}`,
	status,
	trigger_kind: "manual",
	created_at: "2026-09-28T10:00:00Z",
	has_pdf: status !== "pending",
	deliveries: [],
});

const renderDialog = () =>
	render(
		<QueryClientProvider
			client={
				new QueryClient({ defaultOptions: { queries: { retry: false } } })
			}
		>
			<ToastProvider>
				<ReportArchiveDialog
					report={{ id: "r1", name: "Weekly", archive_keep: 24 }}
					onClose={() => {}}
				/>
			</ToastProvider>
		</QueryClientProvider>,
	);

afterEach(() => {
	vi.useRealTimers();
	vi.clearAllMocks();
});

describe("ReportArchiveDialog", () => {
	it("polls the archive while a run is pending", async () => {
		vi.useFakeTimers({ shouldAdvanceTime: true });
		notificationsAPI.listReportArchive.mockResolvedValue({
			data: [run("pending")],
		});
		renderDialog();
		await waitFor(() =>
			expect(screen.getByText("pending")).toBeInTheDocument(),
		);
		await vi.advanceTimersByTimeAsync(6000);
		expect(
			notificationsAPI.listReportArchive.mock.calls.length,
		).toBeGreaterThan(1);
	});

	it("does not poll when every run is finished", async () => {
		vi.useFakeTimers({ shouldAdvanceTime: true });
		notificationsAPI.listReportArchive.mockResolvedValue({
			data: [run("completed")],
		});
		renderDialog();
		await waitFor(() =>
			expect(screen.getByText("completed")).toBeInTheDocument(),
		);
		await vi.advanceTimersByTimeAsync(12000);
		expect(notificationsAPI.listReportArchive).toHaveBeenCalledTimes(1);
	});
});
