import React from "react";
export interface SwitchProps { checked: boolean; onChange: (checked: boolean) => void; label: string; disabled?: boolean; }
export declare function Switch(props: SwitchProps): React.ReactElement;
