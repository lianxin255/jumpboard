import React from "react";
export function Dialog({ title, children, footer, onClose }) {
  const panelRef = React.useRef(null);
  const closeRef = React.useRef(onClose);
  closeRef.current = onClose;
  React.useEffect(() => {
    const previous = document.activeElement;
    const panel = panelRef.current;
    const previousInert = [];
    let root = panel;
    while (root?.parentElement && root.parentElement !== document.body) root = root.parentElement;
    [...document.body.children].filter((element) => element !== root && !["SCRIPT", "STYLE", "LINK"].includes(element.tagName)).forEach((element) => {
      previousInert.push([element, element.inert]);
      element.inert = true;
    });
    panel?.focus();
    const onKey = (event) => {
      if (event.key === "Escape") { event.preventDefault(); closeRef.current(); return; }
      if (event.key !== "Tab") return;
      const focusable = [...panel.querySelectorAll('button:not(:disabled), input:not(:disabled), select:not(:disabled), textarea:not(:disabled), [tabindex="0"]')];
      if (!focusable.length) { event.preventDefault(); panel.focus(); return; }
      const first = focusable[0];
      const last = focusable[focusable.length - 1];
      if (event.shiftKey && (document.activeElement === first || document.activeElement === panel)) { event.preventDefault(); last.focus(); }
      else if (!event.shiftKey && document.activeElement === last) { event.preventDefault(); first.focus(); }
    };
    document.addEventListener("keydown", onKey);
    return () => {
      document.removeEventListener("keydown", onKey);
      previousInert.forEach(([element, inert]) => { element.inert = inert; });
      previous?.focus();
    };
  }, []);
  return <div className="jb-dialog-backdrop" onMouseDown={(event) => { if (event.target === event.currentTarget) closeRef.current(); }}>
    <section ref={panelRef} tabIndex={-1} className="jb-dialog" role="dialog" aria-modal="true" aria-label={title}>
      <header className="jb-dialog__header"><h2 className="section-title">{title}</h2><span style={{ flex: 1 }}></span><button className="jb-icon-button" onClick={onClose} aria-label="关闭">×</button></header>
      <div className="jb-dialog__body">{children}</div>
      {footer && <footer className="jb-dialog__footer">{footer}</footer>}
    </section>
  </div>;
}
