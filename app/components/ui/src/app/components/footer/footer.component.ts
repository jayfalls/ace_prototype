import { Component, OnInit } from '@angular/core';
import { Store } from '@ngrx/store';
import { Observable } from 'rxjs';
import { AppState } from '../../store/state/app.state';
import { selectAppState } from '../../store/selectors/app.selectors';

@Component({
  selector: 'ace-footer',
  templateUrl: './footer.component.html',
  styleUrls: ['./footer.component.scss']
})
export class ACEFooterComponent implements OnInit {
  versionData$: Observable<AppState>;
  version: string = "0";

  constructor(private store: Store) {
      this.versionData$ = this.store.select(selectAppState);
  }

  ngOnInit(): void {
      this.versionData$.subscribe( versionData => this.version = versionData.versionData.version);
  }
}
