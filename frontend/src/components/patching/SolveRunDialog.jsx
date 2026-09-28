import { CheckCheck, Loader2, X } from "lucide-react";
import { useId, useState } from "react";

/**
 * Dialog for marking failed patch runs as solved (or editing the note of a
 * solved run). The note is free text: what was done to fix the failure.
 */
export default function SolveRunDialog({
	open,
	title = "Mark as solved",
	description,
	confirmLabel = "Mark as solved",
	initialNote = "",
	isPending = false,
	onConfirm,
	onClose,
}) {
	const noteId = useId();
	const [note, setNote] = useState(initialNote);
	if (!open) return null;
	const close = () => {
		if (!isPending) onClose();
	};
	return (
		<div className="fixed inset-0 bg-black/50 flex items-center justify-center z-50 p-4">
			<button
				type="button"
				onClick={close}
				className="fixed inset-0 cursor-default"
				aria-label="Close"
			/>
			<div
				role="dialog"
				aria-modal="true"
				className="bg-white dark:bg-secondary-800 rounded-lg shadow-xl max-w-lg w-full mx-4 relative z-10"
			>
				<div className="px-6 py-4 border-b border-secondary-200 dark:border-secondary-600 flex items-center justify-between">
					<h3 className="text-lg font-semibold text-secondary-900 dark:text-white">
						{title}
					</h3>
					<button
						type="button"
						onClick={close}
						disabled={isPending}
						aria-label="Close dialog"
						className="text-secondary-400 hover:text-secondary-600 dark:hover:text-white disabled:opacity-50"
					>
						<X className="h-5 w-5" />
					</button>
				</div>
				<div className="px-6 py-5 space-y-3">
					{description && (
						<p className="text-sm text-secondary-600 dark:text-secondary-300">
							{description}
						</p>
					)}
					<label
						htmlFor={noteId}
						className="block text-sm font-medium text-secondary-700 dark:text-white"
					>
						Note{" "}
						<span className="font-normal text-secondary-500">
							(optional, what fixed it)
						</span>
					</label>
					<textarea
						id={noteId}
						rows={4}
						maxLength={4000}
						value={note}
						onChange={(e) => setNote(e.target.value)}
						placeholder="e.g. ran dpkg --configure -a, removed the stale lock, re-ran the update"
						className="w-full px-3 py-2 bg-white dark:bg-secondary-900 border border-secondary-300 dark:border-secondary-600 rounded-md text-sm text-secondary-900 dark:text-white focus:ring-2 focus:ring-primary-500 focus:border-primary-500 placeholder-secondary-400"
					/>
					<p className="text-xs text-secondary-500">
						The run keeps its output, error and the suggested fixes; only its
						status changes. You can reopen it later.
					</p>
				</div>
				<div className="px-6 py-4 border-t border-secondary-200 dark:border-secondary-600 flex justify-end gap-2">
					<button
						type="button"
						className="btn-outline"
						onClick={close}
						disabled={isPending}
					>
						Cancel
					</button>
					<button
						type="button"
						className="btn-primary inline-flex items-center gap-1.5"
						disabled={isPending}
						onClick={() => onConfirm(note.trim())}
					>
						{isPending ? (
							<Loader2 className="h-4 w-4 animate-spin" />
						) : (
							<CheckCheck className="h-4 w-4" />
						)}
						{confirmLabel}
					</button>
				</div>
			</div>
		</div>
	);
}
