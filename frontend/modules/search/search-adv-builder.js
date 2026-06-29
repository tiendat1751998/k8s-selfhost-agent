/**
 * Enterprise Search Advanced Rules Builder Engine
 */
(function (global) {
  'use strict';

  function createRuleElement() {
    var div = document.createElement('div');
    div.className = 'adv-rule-row';
    div.style.cssText = 'display:flex; gap:var(--space-xs); align-items:center; margin-bottom:4px;';

    var fieldSelect = document.createElement('select');
    fieldSelect.className = 'form-select adv-rule-field';
    fieldSelect.style.cssText = 'width:150px; padding:4px 8px;';
    fieldSelect.innerHTML =
      '<option value="message">Message</option>' +
      '<option value="cluster">Cluster</option>' +
      '<option value="namespace">Namespace</option>' +
      '<option value="severity">Severity</option>';

    var opSelect = document.createElement('select');
    opSelect.className = 'form-select adv-rule-op';
    opSelect.style.cssText = 'width:120px; padding:4px 8px;';
    opSelect.innerHTML =
      '<option value="contains">Contains</option>' +
      '<option value="not_contains">Does Not Contain</option>' +
      '<option value="equals">Equals</option>' +
      '<option value="not_equals">Does Not Equal</option>';

    var valInput = document.createElement('input');
    valInput.type = 'text';
    valInput.className = 'form-select adv-rule-val';
    valInput.placeholder = 'Value...';
    valInput.style.cssText = 'flex:1; padding:4px 8px;';

    var removeBtn = document.createElement('button');
    removeBtn.className = 'btn btn-ghost btn-xs';
    removeBtn.innerHTML = '❌';
    removeBtn.style.padding = '4px 8px';
    removeBtn.addEventListener('click', function () {
      div.remove();
    });

    div.appendChild(fieldSelect);
    div.appendChild(opSelect);
    div.appendChild(valInput);
    div.appendChild(removeBtn);
    return div;
  }

  function createGroupElement() {
    var div = document.createElement('div');
    div.className = 'adv-group-box';
    div.style.cssText = 'border:1px dashed var(--color-hairline); border-radius:var(--rounded-md); padding:var(--space-sm); margin-bottom:8px; background:rgba(255,255,255,0.01);';

    var header = document.createElement('div');
    header.style.cssText = 'display:flex; align-items:center; gap:var(--space-sm); margin-bottom:8px;';
    header.innerHTML =
      '<span style="font-size:11px; font-weight:600;">Match</span>' +
      '<select class="form-select adv-group-match" style="width:80px; padding:2px 4px; font-size:11px;">' +
        '<option value="AND">ALL (AND)</option>' +
        '<option value="OR">ANY (OR)</option>' +
      '</select>' +
      '<span style="font-size:11px; font-weight:600;">of sub-conditions:</span>';

    var removeBtn = document.createElement('button');
    removeBtn.className = 'btn btn-ghost btn-xs';
    removeBtn.innerHTML = 'Remove Group ❌';
    removeBtn.style.cssText = 'margin-left:auto; padding:2px 6px; font-size:11px;';
    removeBtn.addEventListener('click', function () {
      div.remove();
    });
    header.appendChild(removeBtn);

    var rulesContainer = document.createElement('div');
    rulesContainer.className = 'adv-group-rules-container';
    rulesContainer.style.cssText = 'padding-left:12px; display:flex; flex-direction:column; gap:4px;';

    var actions = document.createElement('div');
    actions.style.cssText = 'margin-top:8px; display:flex; gap:var(--space-xs);';

    var addRuleBtn = document.createElement('button');
    addRuleBtn.className = 'btn btn-ghost btn-xs';
    addRuleBtn.textContent = '+ Add Sub-Rule';
    addRuleBtn.style.padding = '2px 6px';
    addRuleBtn.addEventListener('click', function () {
      rulesContainer.appendChild(createRuleElement());
    });

    actions.appendChild(addRuleBtn);
    div.appendChild(header);
    div.appendChild(rulesContainer);
    div.appendChild(actions);

    rulesContainer.appendChild(createRuleElement());
    return div;
  }

  function compileRules(container) {
    if (!container) return [];
    var rules = [];
    container.childNodes.forEach(function (node) {
      if (node.classList && node.classList.contains('adv-rule-row')) {
        var field = node.querySelector('.adv-rule-field').value;
        var op = node.querySelector('.adv-rule-op').value;
        var val = node.querySelector('.adv-rule-val').value.trim();
        rules.push({
          type: 'rule',
          field: field,
          op: op,
          val: val
        });
      } else if (node.classList && node.classList.contains('adv-group-box')) {
        var match = node.querySelector('.adv-group-match').value;
        var subContainer = node.querySelector('.adv-group-rules-container');
        var subRules = compileRules(subContainer);
        rules.push({
          type: 'group',
          match: match,
          rules: subRules
        });
      }
    });
    return rules;
  }

  function evaluateCompiledRules(item, compiledRules, matchType) {
    if (compiledRules.length === 0) return true;

    var isAnd = matchType === 'AND';
    var results = [];

    compiledRules.forEach(function (cond) {
      if (cond.type === 'rule') {
        var fieldVal = '';
        if (cond.field === 'message') {
          fieldVal = item.name;
        } else if (cond.field === 'cluster') {
          fieldVal = item.cluster;
        } else if (cond.field === 'namespace') {
          fieldVal = item.namespace;
        } else if (cond.field === 'severity') {
          if (item.name.indexOf('ERROR') >= 0 || item.name.indexOf('restarted') >= 0 || item.name.indexOf('NotReady') >= 0) {
            fieldVal = 'ERROR';
          } else if (item.name.indexOf('WARN') >= 0) {
            fieldVal = 'WARN';
          } else {
            fieldVal = 'INFO';
          }
        }

        var matches = false;
        var fVal = (fieldVal || '').toLowerCase();
        var qVal = (cond.val || '').toLowerCase();

        switch (cond.op) {
          case 'contains':
            matches = fVal.indexOf(qVal) >= 0;
            break;
          case 'not_contains':
            matches = fVal.indexOf(qVal) < 0;
            break;
          case 'equals':
            matches = fVal === qVal;
            break;
          case 'not_equals':
            matches = fVal !== qVal;
            break;
        }
        results.push(matches);
      } else if (cond.type === 'group') {
        var groupMatches = evaluateCompiledRules(item, cond.rules, cond.match);
        results.push(groupMatches);
      }
    });

    if (isAnd) {
      return results.every(function (r) { return r; });
    } else {
      return results.some(function (r) { return r; });
    }
  }

  function loadRulesIntoDOM(rules, container) {
    if (!container || !rules) return;
    container.innerHTML = '';
    rules.forEach(function (cond) {
      if (cond.type === 'rule') {
        var el = createRuleElement();
        el.querySelector('.adv-rule-field').value = cond.field;
        el.querySelector('.adv-rule-op').value = cond.op;
        el.querySelector('.adv-rule-val').value = cond.val;
        container.appendChild(el);
      } else if (cond.type === 'group') {
        var el = createGroupElement();
        el.querySelector('.adv-group-match').value = cond.match;
        var subContainer = el.querySelector('.adv-group-rules-container');
        loadRulesIntoDOM(cond.rules, subContainer);
        container.appendChild(el);
      }
    });
  }

  global.SearchAdvBuilder = {
    createRuleElement: createRuleElement,
    createGroupElement: createGroupElement,
    compileRules: compileRules,
    evaluateCompiledRules: evaluateCompiledRules,
    loadRulesIntoDOM: loadRulesIntoDOM
  };

})(window);
