
(function(){var p="system";try{p=localStorage.getItem("fabric-explorer-theme")||"system"}catch(e){}var t=p==="dark"||p==="light"?p:(window.matchMedia("(prefers-color-scheme: dark)").matches?"dark":"light");document.documentElement.dataset.theme=t;document.documentElement.style.colorScheme=t})();
