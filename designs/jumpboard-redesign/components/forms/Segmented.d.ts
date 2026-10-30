import React from "react";
export interface SegmentedProps { options: string[]; value: string; onChange: (value: string) => void; label: string; }
export declare function Segmented(props: SegmentedProps): React.ReactElement;
