/**
 * Shared Advanced Rules Builder Engine UI & Evaluation Helper
 */
(function (global) {
  'use strict';

  function RulesBuilder(config) {
    this.fields = config.fields || [];
    this.operators = config.operators || [];
    this.resolveFieldVal = config.resolveFieldVal;
    this.evaluateFieldRule = config.evaluateFieldRule;
  }

  RulesBuilder.prototype.createRuleElement = function () {
    var div = document.createElement('div');
    div.className = 'adv-rule-row';
    div.style.cssText = 'display:flex; gap:var(--space-xs); align-items:center; margin-bottom:4px;';

    var fieldSelect = document.createElement('select');
    fieldSelect.className = 'form-select adv-rule-field';
    fieldSelect.style.cssText = 'width:150px; padding:4px 8px;';
    
    var fieldsHTML = '';
    var esc = (global.Security && global.Security.escapeHTML) || function(s) { return s; };
    this.fields.forEach(function (f) {
      fieldsHTML += '<option value="' + esc(f.value) + '">' + esc(f.label) + '</option>';
    });
    fieldSelect.innerHTML = fieldsHTML;

    var opSelect = document.createElement('select');
    opSelect.className = 'form-select adv-rule-op';
    opSelect.style.cssText = 'width:120px; padding:4px 8px;';
    
    var opsHTML = '';
    this.operators.forEach(function (op) {
      opsHTML += '<option value="' + esc(op.value) + '">' + esc(op.label) + '</option>';
    });
    opSelect.innerHTML = opsHTML;

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
  };

  RulesBuilder.prototype.createGroupElement = function () {
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
    
    var self = this;
    addRuleBtn.addEventListener('click', function () {
      rulesContainer.appendChild(self.createRuleElement());
    });

    actions.appendChild(addRuleBtn);
    div.appendChild(header);
    div.appendChild(rulesContainer);
    div.appendChild(actions);

    rulesContainer.appendChild(this.createRuleElement());
    return div;
  };

  RulesBuilder.prototype.compileRules = function (container) {
    if (!container) return [];
    var rules = [];
    var self = this;
    try {
      container.childNodes.forEach(function (node) {
        if (node.classList && node.classList.contains('adv-rule-row')) {
          var fieldEl = node.querySelector('.adv-rule-field');
          var opEl = node.querySelector('.adv-rule-op');
          var valEl = node.querySelector('.adv-rule-val');
          if (!fieldEl || !opEl || !valEl) {
            throw new Error('Missing rule input element');
          }
          var field = fieldEl.value;
          var op = opEl.value;
          var val = valEl.value.trim();
          rules.push({
            type: 'rule',
            field: field,
            op: op,
            val: val
          });
        } else if (node.classList && node.classList.contains('adv-group-box')) {
          var matchEl = node.querySelector('.adv-group-match');
          var subContainer = node.querySelector('.adv-group-rules-container');
          if (!matchEl || !subContainer) {
            throw new Error('Missing group elements');
          }
          var match = matchEl.value;
          var subRules = self.compileRules(subContainer);
          rules.push({
            type: 'group',
            match: match,
            rules: subRules
          });
        }
      });
    } catch (e) {
      console.error('Failed to compile rules:', e);
      alert('Error: Rules compilation failed. Please check the rule configuration.');
      throw e;
    }
    return rules;
  };

  RulesBuilder.prototype.evaluateCompiledRules = function (item, compiledRules, matchType) {
    if (!compiledRules || compiledRules.length === 0) return true;

    var isAnd = matchType === 'AND';
    var results = [];
    var self = this;

    compiledRules.forEach(function (cond) {
      if (cond.type === 'rule') {
        var matches = false;
        if (self.evaluateFieldRule) {
          matches = self.evaluateFieldRule(item, cond.field, cond.op, cond.val);
        } else {
          var fieldVal = self.resolveFieldVal ? self.resolveFieldVal(item, cond.field) : (item[cond.field] || '');
          var fVal = String(fieldVal).toLowerCase();
          var qVal = String(cond.val).toLowerCase();

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
            default:
              matches = false;
          }
        }
        results.push(matches);
      } else if (cond.type === 'group') {
        var groupMatches = self.evaluateCompiledRules(item, cond.rules, cond.match);
        results.push(groupMatches);
      }
    });

    if (isAnd) {
      return results.every(function (r) { return r; });
    } else {
      return results.some(function (r) { return r; });
    }
  };

  RulesBuilder.prototype.loadRulesIntoDOM = function (rules, container) {
    if (!container || !rules) return;
    container.innerHTML = '';
    var self = this;
    rules.forEach(function (cond) {
      if (cond.type === 'rule') {
        var el = self.createRuleElement();
        el.querySelector('.adv-rule-field').value = cond.field;
        el.querySelector('.adv-rule-op').value = cond.op;
        el.querySelector('.adv-rule-val').value = cond.val;
        container.appendChild(el);
      } else if (cond.type === 'group') {
        var el = self.createGroupElement();
        el.querySelector('.adv-group-match').value = cond.match;
        var subContainer = el.querySelector('.adv-group-rules-container');
        self.loadRulesIntoDOM(cond.rules, subContainer);
        container.appendChild(el);
      }
    });
  };

  global.RulesBuilder = RulesBuilder;

})(window);
