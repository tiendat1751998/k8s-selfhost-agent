(function(global) {
  'use strict';

  class ViewLoader {
    static templateCache = new Map();

    static async loadTemplate(templateUrl, targetContainerId = 'main-content') {
      const container = document.getElementById(targetContainerId);
      if (!container) {
        console.error(`Container #${targetContainerId} not found`);
        return null;
      }

      let html = this.templateCache.get(templateUrl);
      if (!html) {
        try {
          const response = await fetch(templateUrl);
          if (!response.ok) throw new Error(`HTTP error! status: ${response.status}`);
          html = await response.text();
          this.templateCache.set(templateUrl, html);
        } catch (error) {
          console.error(`Error loading template ${templateUrl}:`, error);
          return null;
        }
      }

      // Hide active sections
      const activeSections = container.querySelectorAll('.section.active');
      activeSections.forEach(sec => sec.classList.remove('active'));

      const tempDiv = document.createElement('div');
      tempDiv.innerHTML = html.trim();
      const sectionElement = tempDiv.firstElementChild;

      if (sectionElement && sectionElement.tagName.toLowerCase() === 'section') {
        const sectionId = sectionElement.id;
        let existingSection = document.getElementById(sectionId);
        
        if (!existingSection) {
          container.appendChild(sectionElement);
          existingSection = sectionElement;
        }
        
        existingSection.classList.add('active');
        return existingSection;
      } else {
        console.error(`Template ${templateUrl} does not have a top-level <section> element`);
        return null;
      }
    }
  }

  global.ViewLoader = ViewLoader;

})(window);
