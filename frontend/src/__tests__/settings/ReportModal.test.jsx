import { render, screen } from "@testing-library/react";
import { describe, expect, it } from "vitest";
import { ToastProvider } from "../../contexts/ToastContext";
import ReportModal from "../../pages/settings/ReportModal";

const renderModal = (props = {}) =>
	render(
		<ToastProvider>
			<ReportModal
				isOpen
				onClose={() => {}}
				onSave={() => {}}
				editingReport={null}
				destinations={[]}
				hostGroups={[]}
				hosts={null}
				isPending={false}
				{...props}
			/>
		</ToastProvider>,
	);

describe("ReportModal", () => {
	it("disables Cancel and Close while a save is pending", () => {
		renderModal({ isPending: true });
		expect(screen.getByRole("button", { name: "Cancel" })).toBeDisabled();
		expect(screen.getByRole("button", { name: "Close" })).toBeDisabled();
	});

	it("lists a non-internal destination with its channel icon", () => {
		renderModal({
			destinations: [
				{
					id: "internal-alerts",
					channel_type: "internal",
					display_name: "Internal Alerts",
					enabled: true,
				},
				{
					id: "smtp-1",
					channel_type: "email",
					display_name: "Patch Management Report",
					enabled: true,
				},
			],
		});
		expect(screen.getByText("Patch Management Report")).toBeInTheDocument();
		expect(screen.queryByText("Internal Alerts")).not.toBeInTheDocument();
	});

	it("links every single-field label to its control", () => {
		renderModal();
		expect(screen.getByLabelText(/Report name/)).toBeInstanceOf(
			HTMLInputElement,
		);
		expect(screen.getByLabelText("Language")).toBeInstanceOf(HTMLSelectElement);
		expect(screen.getByLabelText("Activity period")).toBeInstanceOf(
			HTMLSelectElement,
		);
		expect(screen.getByLabelText("Top rows per section")).toBeInstanceOf(
			HTMLInputElement,
		);
		expect(screen.getByLabelText(/Archive retention/)).toBeInstanceOf(
			HTMLInputElement,
		);
	});
});
