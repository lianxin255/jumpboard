import React from "react";
export interface DialogProps { title: string; children: React.ReactNode; footer?: React.ReactNode; onClose: () => void; }
export declare function Dialog(props: DialogProps): React.ReactElement;
