import { render, screen } from "@testing-library/react";
import { describe, expect, it } from "vitest";
import { PatchRunStatusBadge } from "../../components/PatchRunStatusBadge";

describe("PatchRunStatusBadge", () => {
	it("labels a solved run", () => {
		render(<PatchRunStatusBadge run={{ status: "solved" }} />);
		expect(screen.getByText("Solved")).toBeInTheDocument();
	});
});
