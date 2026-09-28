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
