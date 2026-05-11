(function() {
  if (window.__callCheckerStarted) return;
  window.__callCheckerStarted = true;

  var tabId = window.__CALL_CHECKER_TAB_ID || 'unknown';
  console.log('[CALL_STATUS] Checker started for ' + tabId);

  var CHECK_INTERVAL = 10000;
  var INITIAL_DELAY = 10000;

  var BALE_LEAVE_SELECTOR = '[data-testid="leave-call-button"]';

  var checkCallStatus = function() {
    var btn = document.querySelector(BALE_LEAVE_SELECTOR);
    var status = btn ? 'active' : 'inactive';
    console.log('[CALL_STATUS] ' + tabId + ':' + status);
  };

  setTimeout(function() {
    checkCallStatus();
    setInterval(checkCallStatus, CHECK_INTERVAL);
  }, INITIAL_DELAY);
})();
