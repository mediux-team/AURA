"use client";
import React, {
	useCallback,
	useEffect,
	useState,
	useRef,
	useMemo,
} from "react";
import { DBMediaItemWithPosterSets } from "@/types/databaseSavedSet";
import { fetchAllItemsFromDB } from "@/services/api.db";
import Loader from "@/components/ui/loader";
import ErrorMessage from "@/components/ui/error-message";
import SavedSetsCard from "@/components/ui/saved-sets-cards";
import { Button } from "@/components/ui/button";
import { RefreshCcw as RefreshIcon } from "lucide-react";
import { cn } from "@/lib/utils";
import { useHomeSearchStore } from "@/lib/homeSearchStore";
import { searchMediaItems } from "@/hooks/searchMediaItems";
import { Badge } from "@/components/ui/badge";

const SavedSetsPage: React.FC = () => {
	const [savedSets, setSavedSets] = useState<DBMediaItemWithPosterSets[]>([]);
	const [loading, setLoading] = useState(true);
	const [error, setError] = useState(false);
	const [errorMessage, setErrorMessage] = useState<string>("");
	const isFetchingRef = useRef(false);
	const { searchQuery } = useHomeSearchStore();
	const [filterAutoDownloadOnly, setFilterAutoDownloadOnly] = useState(false);

	const fetchSavedSets = useCallback(async () => {
		if (isFetchingRef.current) return;
		isFetchingRef.current = true;
		try {
			const resp = await fetchAllItemsFromDB();
			if (resp.status !== "success") {
				throw new Error(resp.message);
			}
			const sets = resp.data;
			if (!sets) {
				throw new Error("No sets found");
			}
			setSavedSets(sets);
		} catch (error) {
			setError(true);
			setErrorMessage(
				error instanceof Error
					? error.message
					: "An unknown error occurred"
			);
		} finally {
			setLoading(false);
			isFetchingRef.current = false;
		}
	}, []);

	useEffect(() => {
		if (typeof window !== "undefined") {
			// Safe to use document here.
			document.title = "Aura | Saved Sets";
		}
		fetchSavedSets();
	}, [fetchSavedSets]);

	// This useMemo will first filter the savedSets using your search logic,
	// then sort the resulting array from newest to oldest using the LastDownloaded values.
	const filteredAndSortedSavedSets = useMemo(() => {
		let filtered = savedSets;

		if (searchQuery.trim() !== "") {
			const mediaItems = savedSets.map((set) => set.MediaItem);
			const filteredMediaItems = searchMediaItems(
				mediaItems,
				searchQuery
			);
			const filteredKeys = new Set(
				filteredMediaItems.map((item) => item.RatingKey)
			);
			filtered = savedSets.filter((set) =>
				filteredKeys.has(set.MediaItem.RatingKey)
			);
		}

		if (filterAutoDownloadOnly) {
			filtered = filtered.filter(
				(set) =>
					set.PosterSets &&
					set.PosterSets.some((ps) => ps.AutoDownload === true)
			);
		}

		const sorted = filtered.slice().sort((a, b) => {
			const getMaxDownloadTimestamp = (
				set: DBMediaItemWithPosterSets
			) => {
				if (!set.PosterSets || set.PosterSets.length === 0) return 0;
				return set.PosterSets.reduce((max, ps) => {
					const time = new Date(ps.LastDownloaded).getTime();
					return time > max ? time : max;
				}, 0);
			};

			const aMax = getMaxDownloadTimestamp(a);
			const bMax = getMaxDownloadTimestamp(b);
			return bMax - aMax;
		});

		return sorted;
	}, [savedSets, searchQuery, filterAutoDownloadOnly]);

	if (loading) {
		return <Loader message="Loading saved sets..." />;
	}

	if (error) {
		return (
			<div className="flex flex-col items-center p-6 gap-4">
				<ErrorMessage message={errorMessage} />
			</div>
		);
	}

	return (
		<div className="container mx-auto p-4 min-h-screen flex flex-col items-center">
			<Badge
				key={"filter-auto-download-only"}
				className="cursor-pointer mb-4"
				variant={filterAutoDownloadOnly ? "default" : "outline"}
				onClick={() => {
					setFilterAutoDownloadOnly(!filterAutoDownloadOnly);
				}}
			>
				{filterAutoDownloadOnly ? "AutoDownload Only" : "All Items"}
			</Badge>

			<div className="w-full grid grid-cols-1 md:grid-cols-2 lg:grid-cols-3 xl:grid-cols-4 2xl:grid-cols-5 gap-2">
				{filteredAndSortedSavedSets.length > 0 ? (
					filteredAndSortedSavedSets.map((savedSet) => (
						<SavedSetsCard
							key={savedSet.MediaItem.RatingKey}
							savedSet={savedSet}
							onUpdate={fetchSavedSets}
						/>
					))
				) : (
					<p className="text-muted-foreground">
						No saved sets found.
					</p>
				)}
			</div>

			<Button
				variant="outline"
				size="sm"
				className={cn(
					"fixed z-100 right-3 bottom-10 sm:bottom-15 rounded-full shadow-lg transition-all duration-300 bg-background border-primary-dynamic text-primary-dynamic hover:bg-primary-dynamic hover:text-primary cursor-pointer"
				)}
				onClick={() => fetchSavedSets()}
				aria-label="refresh"
			>
				<RefreshIcon className="h-3 w-3 mr-1" />
				<span className="text-xs hidden sm:inline">Refresh</span>
			</Button>
		</div>
	);
};

export default SavedSetsPage;
