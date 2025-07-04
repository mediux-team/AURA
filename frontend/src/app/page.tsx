"use client";

import { fetchMediaServerLibrarySectionItems, fetchMediaServerLibrarySections } from "@/services/api.mediaserver";
import { ReturnErrorMessage } from "@/services/api.shared";
import { ArrowDownAZ, ArrowDownZA, ClockArrowDown, ClockArrowUp } from "lucide-react";

import { useCallback, useEffect, useRef, useState } from "react";

import { CustomPagination } from "@/components/shared/custom-pagination";
import { ErrorMessage } from "@/components/shared/error-message";
import { SelectItemsPerPage } from "@/components/shared/items-per-page-select";
import HomeMediaItemCard from "@/components/shared/media-item-card";
import { RefreshButton } from "@/components/shared/refresh-button";
import { SortControl } from "@/components/shared/sort-control";
import { Badge } from "@/components/ui/badge";
import { Label } from "@/components/ui/label";
import { Progress } from "@/components/ui/progress";
import { ToggleGroup } from "@/components/ui/toggle-group";

import { log } from "@/lib/logger";
import { useHomePageStore } from "@/lib/pageHomeStore";
import { usePaginationStore } from "@/lib/paginationStore";
import { useSearchQueryStore } from "@/lib/searchQueryStore";
import { librarySectionsStorage } from "@/lib/storage";
import { homePageStorage } from "@/lib/storage";

import { searchMediaItems } from "@/hooks/searchMediaItems";

import { APIResponse } from "@/types/apiResponse";
import { LibrarySection, MediaItem } from "@/types/mediaItem";

const CACHE_DURATION = 60 * 60 * 1000;

export default function Home() {
	const isMounted = useRef(false);
	if (typeof window !== "undefined") {
		// Safe to use document here.
		document.title = "aura | Home";
	}
	// -------------------------------
	// States
	// -------------------------------
	// Search
	const { searchQuery } = useSearchQueryStore();
	const prevSearchQuery = useRef(searchQuery);

	// Loading & Error
	const [error, setError] = useState<APIResponse<unknown> | null>(null);
	const [fullyLoaded, setFullyLoaded] = useState<boolean>(false);

	// Library sections & progress
	const [librarySections, setLibrarySections] = useState<LibrarySection[]>([]);
	const [sectionProgress, setSectionProgress] = useState<{
		[key: string]: { loaded: number; total: number };
	}>({});

	// Filtering & Pagination
	const { filteredLibraries, setFilteredLibraries, filterOutInDB, setFilterOutInDB } = useHomePageStore();
	const [filteredItems, setFilteredItems] = useState<MediaItem[]>([]);
	const { currentPage, setCurrentPage } = usePaginationStore();
	const { itemsPerPage } = usePaginationStore();

	// State to track the selected sorting option
	const { sortOption, setSortOption } = useHomePageStore();
	const { sortOrder, setSortOrder } = useHomePageStore();

	// -------------------------------
	// Derived values
	// -------------------------------
	const paginatedItems = filteredItems.slice((currentPage - 1) * itemsPerPage, currentPage * itemsPerPage);
	const totalPages = Math.ceil(filteredItems.length / itemsPerPage);

	// Set sortOption to "dateAdded" if its not title or dateUpdated or dateAdded or dateReleased
	if (
		sortOption !== "title" &&
		sortOption !== "dateUpdated" &&
		sortOption !== "dateAdded" &&
		sortOption !== "dateReleased"
	) {
		setSortOption("dateAdded");
	}

	// Fetch data from cache or API
	const getMediaItems = useCallback(async (useCache: boolean) => {
		if (isMounted.current) return;
		// Reset progress state before starting a new fetch
		setSectionProgress({});
		setFullyLoaded(false);
		isMounted.current = true;
		try {
			let sections: LibrarySection[] = [];

			// If cache is allowed, try loading from librarySectionsStorage
			if (useCache) {
				log("Home Page - Attempting to load sections from cache");
				// Get all cached sections
				const cachedSections: {
					data: LibrarySection;
					timestamp: number;
				}[] = (
					await librarySectionsStorage.keys().then((keys) =>
						Promise.all(
							keys.map((key) =>
								librarySectionsStorage.getItem<{
									data: LibrarySection;
									timestamp: number;
								}>(key)
							)
						)
					)
				).filter((section): section is { data: LibrarySection; timestamp: number } => section !== null);

				if (cachedSections && cachedSections.length > 0) {
					// Filter valid cached sections
					const validSections = cachedSections.filter(
						(section) => Date.now() - section.timestamp < CACHE_DURATION
					);

					if (validSections.length > 0) {
						sections = validSections.map((s) => s.data);
						setLibrarySections(sections.sort((a, b) => a.Title.localeCompare(b.Title)));
						setFullyLoaded(true);
						log("Home Page - Using cached sections", validSections);
						return;
					}
				}

				// Clear invalid cache
				if (sections.length === 0) {
					await librarySectionsStorage.clear();
				}
			}

			setFullyLoaded(false);

			// Clear the cache
			librarySectionsStorage.clear();

			// If sections were not loaded from cache, fetch them from the API.
			if (sections.length === 0) {
				// Fetch sections from API if not in cache
				const response = await fetchMediaServerLibrarySections();
				if (response.status === "error") {
					setError(response);
					setFullyLoaded(true);
					return;
				}

				sections = response.data || [];
				if (!sections || sections.length === 0) {
					setError(ReturnErrorMessage<unknown>(new Error("No sections found, please check the logs.")));
					return;
				}

				// Initialize and fetch items for each section
				sections.forEach((section) => (section.MediaItems = []));
				setLibrarySections(sections.sort((a, b) => a.Title.localeCompare(b.Title)));
			}

			// Process each section concurrently
			await Promise.all(
				sections.map(async (section, idx) => {
					let itemsFetched = 0;
					let totalSize = Infinity;
					let allItems: LibrarySection["MediaItems"] = [];

					while (itemsFetched < totalSize) {
						const itemsResponse = await fetchMediaServerLibrarySectionItems(section, itemsFetched);
						if (itemsResponse.status === "error") {
							setError(itemsResponse);
							return;
						}

						const data = itemsResponse.data;
						const items = data?.MediaItems || [];
						allItems = allItems.concat(items);
						if (totalSize === Infinity) {
							totalSize = data?.TotalSize ?? 0;
						}
						itemsFetched += items.length;
						// Update the progress state for this section:
						setSectionProgress((prev) => ({
							...prev,
							[section.ID]: {
								loaded: itemsFetched,
								total: totalSize,
							},
						}));
						if (items.length === 0) {
							break;
						}
					}
					// Update section with fetched media items.
					section.MediaItems = allItems;
					section.TotalSize = totalSize;
					setLibrarySections((prev) => {
						const updated = [...prev];
						updated[idx] = section;
						return updated;
					});

					// Cache using storage
					await librarySectionsStorage.setItem(`${section.Title}`, {
						data: section,
						timestamp: Date.now(),
					});
				})
			);

			log("Home Page - Sections fetched successfully", sections);
			setFullyLoaded(true);
		} catch (error) {
			setError(ReturnErrorMessage<unknown>(error));
		} finally {
			isMounted.current = false;
		}
	}, []);

	useEffect(() => {
		getMediaItems(true);
	}, [getMediaItems]);

	useEffect(() => {
		if (searchQuery !== prevSearchQuery.current) {
			setCurrentPage(1);
			prevSearchQuery.current = searchQuery;
		}
	}, [searchQuery, setCurrentPage]);

	// Filter items based on the search query
	useEffect(() => {
		let items = librarySections.flatMap((section) => section.MediaItems || []);

		// Sort items by Title
		if (sortOption === "title") {
			if (sortOrder === "asc") {
				items.sort((a, b) => a.Title.localeCompare(b.Title));
			} else if (sortOrder === "desc") {
				items.sort((a, b) => b.Title.localeCompare(a.Title));
			}
		} else if (sortOption === "dateUpdated") {
			if (sortOrder === "asc") {
				items.sort((a, b) => (a.UpdatedAt ?? 0) - (b.UpdatedAt ?? 0));
			} else if (sortOrder === "desc") {
				items.sort((a, b) => (b.UpdatedAt ?? 0) - (a.UpdatedAt ?? 0));
			}
		} else if (sortOption === "dateAdded") {
			if (sortOrder === "asc") {
				items.sort((a, b) => (a.AddedAt ?? 0) - (b.AddedAt ?? 0));
			} else if (sortOrder === "desc") {
				items.sort((a, b) => (b.AddedAt ?? 0) - (a.AddedAt ?? 0));
			}
		} else if (sortOption === "dateReleased") {
			if (sortOrder === "asc") {
				items.sort((a, b) => (a.ReleasedAt ?? 0) - (b.ReleasedAt ?? 0));
			} else if (sortOrder === "desc") {
				items.sort((a, b) => (b.ReleasedAt ?? 0) - (a.ReleasedAt ?? 0));
			}
		}

		// Filter by selected libraries
		if (filteredLibraries.length > 0) {
			items = items.filter((item) => filteredLibraries.includes(item.LibraryTitle));
		}

		// Filter out items already in the DB
		if (filterOutInDB) {
			items = items.filter((item) => !item.ExistInDatabase);
		}

		// Filter out items by search
		const filteredItems = searchMediaItems(items, searchQuery);
		setFilteredItems(filteredItems);

		// Store the filtered items in local storage
		homePageStorage.setItem("filtered-sorted-items", { data: filteredItems });
	}, [librarySections, filteredLibraries, searchQuery, filterOutInDB, sortOption, sortOrder]);

	if (error) {
		return <ErrorMessage error={error} />;
	}

	const hasUpdatedAt = paginatedItems.some((item) => item.UpdatedAt !== undefined && item.UpdatedAt !== null);

	return (
		<div className="min-h-screen px-8 pb-20 sm:px-20">
			{!fullyLoaded && librarySections.length > 0 && (
				<div className="mb-4">
					{librarySections.map((section) => {
						// Retrieve progress info for this section
						const progressInfo = sectionProgress[section.ID];
						const percentage =
							progressInfo && progressInfo.total > 0
								? Math.min((progressInfo.loaded / progressInfo.total) * 100, 100)
								: 0;

						// Render progress UI only if the percentage is not 100
						if (Math.round(percentage) !== 100) {
							return (
								<div key={section.ID} className="mb-2">
									<Label className="text-lg font-semibold">Loading {section.Title}</Label>
									<Progress value={percentage} className="mt-1" />
									<span className="ml-2 text-sm text-muted-foreground">
										{Math.round(percentage)}%
									</span>
								</div>
							);
						}
					})}
				</div>
			)}
			{/* Filter Section*/}
			<div className="flex flex-col sm:flex-row mb-4 mt-2">
				{/* Label */}
				<Label htmlFor="library-filter" className="text-lg font-semibold mb-2 sm:mb-0 sm:mr-4">
					Filters:
				</Label>

				{/* ToggleGroup */}
				<ToggleGroup
					type="multiple"
					className="flex flex-wrap sm:flex-nowrap gap-2"
					value={filteredLibraries}
					onValueChange={setFilteredLibraries}
				>
					{librarySections.map((section) => (
						<Badge
							key={section.ID}
							className="cursor-pointer text-sm"
							variant={filteredLibraries.includes(section.Title) ? "default" : "outline"}
							onClick={() => {
								if (filteredLibraries.includes(section.Title)) {
									setFilteredLibraries(
										filteredLibraries.filter((lib: string) => lib !== section.Title)
									);
									setCurrentPage(1);
								} else {
									setFilteredLibraries([...filteredLibraries, section.Title]);
									setCurrentPage(1);
								}
							}}
						>
							{section.Title}
						</Badge>
					))}

					<Badge
						key={"filter-out-in-db"}
						className="cursor-pointer text-sm"
						variant={filterOutInDB ? "default" : "outline"}
						onClick={() => {
							setFilterOutInDB(!filterOutInDB);
							setCurrentPage(1);
						}}
					>
						{filterOutInDB ? "Items Not in DB" : "All Items"}
					</Badge>
				</ToggleGroup>
			</div>
			{/* Sorting controls */}
			<SortControl
				options={[
					{
						value: "dateAdded",
						label: "Date Added",
						ascIcon: <ClockArrowUp />,
						descIcon: <ClockArrowDown />,
					},
					// Conditionally include "dateUpdated"
					...(hasUpdatedAt
						? [
								{
									value: "dateUpdated",
									label: "Date Updated",
									ascIcon: <ClockArrowUp />,
									descIcon: <ClockArrowDown />,
								},
							]
						: []),
					{
						value: "dateReleased",
						label: "Date Released",
						ascIcon: <ClockArrowUp />,
						descIcon: <ClockArrowDown />,
					},
					{ value: "title", label: "Title", ascIcon: <ArrowDownAZ />, descIcon: <ArrowDownZA /> },
				]}
				sortOption={sortOption}
				sortOrder={sortOrder}
				setSortOption={(value) => {
					setSortOption(value as "title" | "dateUpdated" | "dateAdded" | "dateReleased");
					if (value === "title") setSortOrder("asc");
					else if (value === "dateUpdated") setSortOrder("desc");
					else if (value === "dateAdded") setSortOrder("desc");
					else if (value === "dateReleased") setSortOrder("desc");
				}}
				setSortOrder={setSortOrder}
			/>
			{/* Items Per Page Selection */}
			<div className="flex items-center mb-4">
				<SelectItemsPerPage setCurrentPage={setCurrentPage} />
			</div>
			{/* Grid of Cards */}
			<div className="w-full grid grid-cols-1 md:grid-cols-2 lg:grid-cols-3 xl:grid-cols-4 2xl:grid-cols-5 gap-4">
				{paginatedItems.length === 0 && fullyLoaded && (searchQuery || filteredLibraries.length > 0) ? (
					<div className="col-span-full text-center text-red-500">
						<ErrorMessage
							error={ReturnErrorMessage<string>(
								`No items found ${searchQuery ? `matching "${searchQuery}"` : ""} in 
								${filteredLibraries.length > 0 ? filteredLibraries.join(", ") : "any library"} 
								${filterOutInDB ? "that are not in the database." : ""}`
							)}
						/>
					</div>
				) : (
					paginatedItems.map((item) => <HomeMediaItemCard key={item.RatingKey} mediaItem={item} />)
				)}
			</div>

			{/* Pagination */}
			<CustomPagination
				currentPage={currentPage}
				totalPages={totalPages}
				setCurrentPage={setCurrentPage}
				scrollToTop={true}
				filterItemsLength={filteredItems.length}
			/>
			{/* Refresh Button */}
			<RefreshButton onClick={() => getMediaItems(false)} />
		</div>
	);
}
