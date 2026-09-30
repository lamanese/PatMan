import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import { fireEvent, render, screen } from "@testing-library/react";
import { MemoryRouter, Route, Routes, useLocation } from "react-router-dom";
import { beforeEach, describe, expect, it, vi } from "vitest";
import { ToastProvider } from "../../contexts/ToastContext";
import Schedules from "../../pages/Schedules";

const auth = { permissions: new Set(), modules: new Set() };

vi.mock("../../contexts/AuthContext", () => ({
	useAuth: () => ({
		hasPermission: (p) => auth.permissions.has(p),
		hasModule: (m) => auth.modules.has(m),
		canManagePatching: () => auth.permissions.has("can_manage_patching"),
	}),
}));

const group = { id: "g1", name: "Linux Servers", host_count: 3 };

vi.mock("../../utils/api", () => ({
	formatDate: (d) => String(d),
	hostGroupsAPI: {
		list: vi.fn(() => Promise.resolve({ data: [group] })),
		getHosts: vi.fn(() => Promise.resolve({ data: [] })),
	},
	patchSchedulesAPI: {
		list: vi.fn(() =>
			Promise.resolve({
				data: [
					{
						id: "p1",
						name: "Weekly Linux patching",
						host_group_id: "g1",
						host_group_name: "Linux Servers",
						schedule_type: "weekly",
						weekday: 0,
						time_of_day: "03:00",
						timezone: "Europe/Zurich",
						enabled: true,
					},
				],
			}),
		),
		create: vi.fn(),
		update: vi.fn(),
		delete: vi.fn(),
	},
	rebootSchedulesAPI: {
		list: vi.fn(() =>
			Promise.resolve({
				data: [
					{
						id: "r1",
						name: "Sunday maintenance reboot",
						host_group_id: "g1",
						host_group_name: "Linux Servers",
						schedule_type: "daily",
						time_of_day: "04:00",
						timezone: "Europe/Zurich",
						enabled: true,
					},
				],
			}),
		),
		create: vi.fn(),
		update: vi.fn(),
		delete: vi.fn(),
	},
}));

const LocationProbe = () => {
	const location = useLocation();
	return (
		<div data-testid="location">{location.pathname + location.search}</div>
	);
};

const renderAt = (url) => {
	const qc = new QueryClient({ defaultOptions: { queries: { retry: false } } });
	return render(
		<QueryClientProvider client={qc}>
			<ToastProvider>
				<MemoryRouter initialEntries={[url]}>
					<Routes>
						<Route
							path="/schedules"
							element={
								<>
									<Schedules />
									<LocationProbe />
								</>
							}
						/>
					</Routes>
				</MemoryRouter>
			</ToastProvider>
		</QueryClientProvider>,
	);
};

describe("Schedules page", () => {
	beforeEach(() => {
		auth.permissions = new Set(["can_manage_patching", "can_reboot_hosts"]);
		auth.modules = new Set(["patching"]);
	});

	it("renders the patch tab by default and switches to reboot via the URL", async () => {
		renderAt("/schedules");
		expect(
			screen.getByRole("heading", { name: "Schedules" }),
		).toBeInTheDocument();
		expect(screen.getByRole("button", { name: /Patch/ })).toBeInTheDocument();
		expect(screen.getByRole("button", { name: /Reboot/ })).toBeInTheDocument();
		expect(
			await screen.findByText("Weekly Linux patching"),
		).toBeInTheDocument();
		expect(
			screen.queryByText("Sunday maintenance reboot"),
		).not.toBeInTheDocument();

		fireEvent.click(screen.getByRole("button", { name: /Reboot/ }));
		expect(
			await screen.findByText("Sunday maintenance reboot"),
		).toBeInTheDocument();
		expect(screen.queryByText("Weekly Linux patching")).not.toBeInTheDocument();
		expect(screen.getByTestId("location")).toHaveTextContent(
			"/schedules?tab=reboot",
		);
	});

	it("opens the tab named in the URL", async () => {
		renderAt("/schedules?tab=reboot");
		expect(
			await screen.findByText("Sunday maintenance reboot"),
		).toBeInTheDocument();
	});

	it("shows only the reboot tab without patch permission and falls back to it", async () => {
		auth.permissions = new Set(["can_reboot_hosts"]);
		renderAt("/schedules?tab=patch");
		expect(
			await screen.findByText("Sunday maintenance reboot"),
		).toBeInTheDocument();
		expect(
			screen.queryByRole("button", { name: /Patch/ }),
		).not.toBeInTheDocument();
		expect(screen.queryByText("Weekly Linux patching")).not.toBeInTheDocument();
	});

	it("hides the patch tab when the patching module is missing", async () => {
		auth.modules = new Set();
		renderAt("/schedules");
		expect(
			await screen.findByText("Sunday maintenance reboot"),
		).toBeInTheDocument();
		expect(
			screen.queryByRole("button", { name: /Patch/ }),
		).not.toBeInTheDocument();
	});

	it("shows an access notice when no tab is available", () => {
		auth.permissions = new Set(["can_manage_patching"]);
		auth.modules = new Set();
		renderAt("/schedules");
		expect(
			screen.getByText(/don't have access to any schedules/),
		).toBeInTheDocument();
	});
});
