import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import { render, screen, waitFor } from "@testing-library/react";
import { BrowserRouter } from "react-router-dom";
import { describe, expect, it, vi } from "vitest";
import Integrations from "../../pages/settings/Integrations";

vi.mock("../../utils/api", () => ({
	default: {
		get: vi.fn((path) =>
			Promise.resolve({ data: path === "/host-groups" ? [] : [] }),
		),
		post: vi.fn(),
		delete: vi.fn(),
	},
	dashboardAPI: { getHosts: vi.fn(() => Promise.resolve({ data: [] })) },
	settingsAPI: {
		get: vi.fn(() => Promise.resolve({ data: {} })),
		getServerUrl: vi.fn(() =>
			Promise.resolve({ data: { server_url: "https://pm.example.com" } }),
		),
	},
	formatDate: (d) => String(d),
}));

// Opening the page (no token has just been created) must never touch a
// freshly created token: am.23 crashed here with "Something went wrong".
describe("Integrations page", () => {
	it("renders without a freshly created token", async () => {
		render(
			<QueryClientProvider
				client={
					new QueryClient({ defaultOptions: { queries: { retry: false } } })
				}
			>
				<BrowserRouter>
					<Integrations />
				</BrowserRouter>
			</QueryClientProvider>,
		);
		await waitFor(() =>
			expect(screen.getAllByText(/Auto-Enrollment/i).length).toBeGreaterThan(0),
		);
	});
});
