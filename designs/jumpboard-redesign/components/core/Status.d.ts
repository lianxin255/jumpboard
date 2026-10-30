import React from "react";
export interface StatusProps { children: React.ReactNode; tone?: "neutral" | "success" | "warning" | "danger"; }
export declare function Status(props: StatusProps): React.ReactElement;
