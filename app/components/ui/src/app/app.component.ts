import { Component } from '@angular/core';
import { RouterOutlet } from '@angular/router';
import { ACESidebarComponent } from './components/sidebar/sidebar.component';

@Component({
  selector: 'app-root',
  imports: [
    ACESidebarComponent
  ],
  templateUrl: './app.component.html',
  styleUrl: './app.component.scss'
})
export class AppComponent {
  title = 'ACE';
}
