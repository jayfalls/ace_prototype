import { Component } from '@angular/core';
import { RouterOutlet } from '@angular/router';

@Component({
  selector: 'app-root',
  imports: [RouterOutlet],
  templateUrl: './root.component.html',
  styleUrl: './root.component.scss'
})
export class RootComponent {
  title = 'ACE';
}
