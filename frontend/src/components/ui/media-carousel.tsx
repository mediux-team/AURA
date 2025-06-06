"use client";

import {
	Carousel,
	CarouselContent,
	CarouselNext,
	CarouselPrevious,
} from "@/components/ui/carousel";

import { PosterSet } from "@/types/posterSets";
import { CarouselShow } from "../carousel-show";
import { CarouselMovie } from "../carousel-movie";
import { MediaItem } from "@/types/mediaItem";
import { ZoomInIcon } from "lucide-react";
import { useRouter } from "next/navigation";
import { Lead } from "./typography";
import { usePosterSetStore } from "@/lib/posterSetStore";
import { useMediaStore } from "@/lib/mediaStore";
import { formatLastUpdatedDate } from "@/helper/formatDate";
import { SetFileCounts } from "../set_file_counts";
import DownloadModalShow from "../download-modal-show";
import DownloadModalMovie from "../download-modal-movie";
import { useState } from "react";

type MediaCarouselProps = {
	set: PosterSet;
	mediaItem: MediaItem;
};

export function MediaCarousel({ set, mediaItem }: MediaCarouselProps) {
	const router = useRouter();

	const { setPosterSet } = usePosterSetStore();
	const { setMediaItem } = useMediaStore();
	const [isDownloadModalOpen, setIsDownloadModalOpen] = useState(false);

	const goToSetPage = () => {
		setPosterSet(set);
		setMediaItem(mediaItem);
		router.push(`/sets/${set.ID}`);
	};

	const goToUserPage = () => {
		router.push(`/user/${set.User.Name}`);
	};

	return (
		<Carousel
			opts={{
				align: "start",
				dragFree: true,
				slidesToScroll: "auto",
			}}
			className="w-full"
		>
			<div className="flex flex-col">
				<div className="flex flex-row items-center">
					<div className="flex flex-row items-center">
						<div
							onClick={() => {
								goToSetPage();
							}}
							className="text-primary-dynamic hover:text-primary cursor-pointer text-md font-semibold"
						>
							{set.Title}
						</div>
					</div>
					<div className="ml-auto flex space-x-2">
						<button
							className="btn"
							onClick={() => {
								goToSetPage();
							}}
						>
							<ZoomInIcon className="mr-2 h-5 w-5 sm:h-7 sm:w-7" />
						</button>
						{mediaItem.Type === "show" ? (
							<button className="btn">
								<DownloadModalShow
									posterSet={set}
									mediaItem={mediaItem}
									open={isDownloadModalOpen}
									onOpenChange={setIsDownloadModalOpen}
								/>
							</button>
						) : mediaItem.Type === "movie" ? (
							<button className="btn">
								<DownloadModalMovie
									posterSet={set}
									mediaItem={mediaItem}
									open={isDownloadModalOpen}
									onOpenChange={setIsDownloadModalOpen}
								/>
							</button>
						) : null}
					</div>
				</div>
				<div className="text-md text-muted-foreground  mb-1">
					By:{" "}
					<span
						onClick={(e) => {
							e.stopPropagation();
							goToUserPage();
						}}
						className="hover:text-primary cursor-pointer"
					>
						{set.User.Name}
					</span>
				</div>
				<Lead className="text-sm text-muted-foreground flex items-center mb-1">
					Last Update:{" "}
					{formatLastUpdatedDate(set.DateUpdated, set.DateCreated)}
				</Lead>

				<SetFileCounts mediaItem={mediaItem} set={set} />
			</div>

			<CarouselContent>
				{mediaItem.Type === "show" ? (
					<CarouselShow set={set as PosterSet} />
				) : mediaItem.Type === "movie" ? (
					<CarouselMovie
						set={set as PosterSet}
						librarySection={mediaItem.LibraryTitle}
					/>
				) : null}
			</CarouselContent>
			<CarouselNext className="right-2 bottom-0" />
			<CarouselPrevious className="right-8 bottom-0" />
		</Carousel>
	);
}
