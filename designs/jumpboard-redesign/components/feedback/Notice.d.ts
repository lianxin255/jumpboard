import React from "react";
export interface NoticeProps { children: React.ReactNode; tone?: "warning" | "danger"; action?: React.ReactNode; }
export declare function Notice(props: NoticeProps): React.ReactElement;
